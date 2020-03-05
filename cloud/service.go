package cloud

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/mail"
	"path/filepath"
	"strings"
	"time"

	"github.com/textileio/go-threads/api/client"

	ipfsfiles "github.com/ipfs/go-ipfs-files"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	fc "github.com/textileio/filecoin/api/client"
	"github.com/textileio/go-threads/broadcast"
	pb "github.com/textileio/textile/cloud/pb"
	c "github.com/textileio/textile/collections"
	"github.com/textileio/textile/dns"
	"github.com/textileio/textile/email"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	loginTimeout = time.Minute * 3
	emailTimeout = time.Second * 10
)

const (
	// bucketSeedName is the name of the seed file used to ensure buckets are unique
	bucketSeedName = ".textilebucketseed"
	// chunkSize for get file requests.
	chunkSize = 1024 * 32
)

// service is a gRPC service for textile.
type service struct {
	collections *c.Collections

	emailClient    *email.Client
	ipfsClient     iface.CoreAPI
	threadsClient  *client.Client
	filecoinClient *fc.Client

	dnsManager *dns.Manager

	gatewayUrl string

	sessionBus    *broadcast.Broadcaster
	sessionSecret string
}

func (s *service) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginReply, error) {
	log.Debugf("received login request")

	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Email address in not valid")
	}
	dev, err := s.collections.Developers.GetOrCreate(ctx, req.Username, req.Email, "@todo")
	if err != nil {
		return nil, err
	}

	secret := getSessionSecret(s.sessionSecret)
	ectx, cancel := context.WithTimeout(ctx, emailTimeout)
	defer cancel()
	if err = s.emailClient.ConfirmAddress(ectx, dev.Email, s.gatewayUrl, secret); err != nil {
		return nil, err
	}
	if !s.awaitVerification(secret) {
		return nil, status.Error(codes.Unauthenticated, "Could not verify email address")
	}

	session, err := s.collections.Sessions.Create(ctx, dev.ID)
	if err != nil {
		return nil, err
	}
	return &pb.LoginReply{
		ID:       dev.ID.Hex(),
		Username: dev.Username,
		Token:    session.Token,
	}, nil
}

func getSessionSecret(secret string) string {
	if secret != "" {
		return secret
	}
	return c.MakeURLSafeToken(32)
}

func (s *service) Logout(ctx context.Context, _ *pb.LogoutRequest) (*pb.LogoutReply, error) {
	log.Debugf("received logout request")

	session, _ := c.SessionFromContext(ctx)
	if err := s.collections.Sessions.Delete(ctx, session.ID); err != nil {
		return nil, err
	}
	return &pb.LogoutReply{}, nil
}

func (s *service) Whoami(ctx context.Context, _ *pb.WhoamiRequest) (*pb.WhoamiReply, error) {
	log.Debugf("received whoami request")

	dev, _ := c.DevFromContext(ctx)
	return &pb.WhoamiReply{
		ID:       dev.ID.Hex(),
		Username: dev.Username,
		Email:    dev.Email,
	}, nil
}

// awaitVerification waits for a dev to verify their email via a sent email.
func (s *service) awaitVerification(secret string) bool {
	listen := s.sessionBus.Listen()
	ch := make(chan struct{})
	timer := time.NewTimer(loginTimeout)
	go func() {
		for i := range listen.Channel() {
			if r, ok := i.(string); ok && r == secret {
				ch <- struct{}{}
			}
		}
	}()
	select {
	case <-ch:
		listen.Discard()
		timer.Stop()
		return true
	case <-timer.C:
		listen.Discard()
		return false
	}
}

func (s *service) AddOrg(ctx context.Context, req *pb.AddOrgRequest) (*pb.GetOrgReply, error) {
	log.Debugf("received add org request")

	dev, _ := c.DevFromContext(ctx)
	org := &c.Org{
		Name: req.Name,
		Members: []c.Member{{
			ID:       dev.ID,
			Username: dev.Username,
			Role:     c.OrgOwner,
		}},
	}
	if err := s.collections.Orgs.Create(ctx, org); err != nil {
		return nil, err
	}
	return orgToPbOrg(org), nil
}

func (s *service) GetOrg(ctx context.Context, _ *pb.GetOrgRequest) (*pb.GetOrgReply, error) {
	log.Debugf("received get org request")

	org, ok := c.OrgFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("org required")
	}
	return orgToPbOrg(org), nil
}

func orgToPbOrg(org *c.Org) *pb.GetOrgReply {
	members := make([]*pb.GetOrgReply_Member, len(org.Members))
	for i, m := range org.Members {
		members[i] = &pb.GetOrgReply_Member{
			ID:       m.ID.Hex(),
			Username: m.Username,
			Role:     m.Role.String(),
		}
	}
	return &pb.GetOrgReply{
		ID:        org.ID.Hex(),
		Name:      org.Name,
		Members:   members,
		CreatedAt: org.CreatedAt.Unix(),
	}
}

func (s *service) ListOrgs(ctx context.Context, _ *pb.ListOrgsRequest) (*pb.ListOrgsReply, error) {
	log.Debugf("received list orgs request")

	dev, _ := c.DevFromContext(ctx)
	orgs, err := s.collections.Orgs.List(ctx, dev.ID)
	if err != nil {
		return nil, err
	}
	list := make([]*pb.GetOrgReply, len(orgs))
	for i, org := range orgs {
		list[i] = orgToPbOrg(&org)
	}
	return &pb.ListOrgsReply{List: list}, nil
}

// @todo: Delete org objects.
func (s *service) RemoveOrg(ctx context.Context, _ *pb.RemoveOrgRequest) (*pb.RemoveOrgReply, error) {
	log.Debugf("received remove org request")

	dev, _ := c.DevFromContext(ctx)
	org, ok := c.OrgFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("org required")
	}
	isOwner, err := s.collections.Orgs.IsOwner(ctx, org.Name, dev.ID)
	if err != nil {
		return nil, err
	}
	if !isOwner {
		return nil, status.Error(codes.PermissionDenied, "User must be an org owner")
	}

	if err = s.collections.Orgs.Delete(ctx, org.ID); err != nil {
		return nil, err
	}
	return &pb.RemoveOrgReply{}, nil
}

func (s *service) InviteToOrg(ctx context.Context, req *pb.InviteToOrgRequest) (*pb.InviteToOrgReply, error) {
	log.Debugf("received invite to org request")

	dev, _ := c.DevFromContext(ctx)
	org, ok := c.OrgFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("org required")
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Email address in not valid")
	}
	invite, err := s.collections.Invites.Create(ctx, org.ID, dev.ID, req.Email)
	if err != nil {
		return nil, err
	}

	ectx, cancel := context.WithTimeout(ctx, emailTimeout)
	defer cancel()
	if err = s.emailClient.InviteAddress(
		ectx, org.Name, dev.Email, req.Email, s.gatewayUrl, invite.Token); err != nil {
		return nil, err
	}
	return &pb.InviteToOrgReply{InviteID: invite.ID.Hex()}, nil
}

// LeaveOrg handles a leave org request.
func (s *service) LeaveOrg(ctx context.Context, _ *pb.LeaveOrgRequest) (*pb.LeaveOrgReply, error) {
	log.Debugf("received leave org request")

	dev, _ := c.DevFromContext(ctx)
	org, ok := c.OrgFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("org required")
	}
	if err := s.collections.Orgs.RemoveMember(ctx, org.Name, dev.ID); err != nil {
		return nil, err
	}
	return &pb.LeaveOrgReply{}, nil
}

// ListBucketPath handles a list bucket path request.
func (s *service) ListBucketPath(ctx context.Context, req *pb.ListBucketPathRequest) (*pb.ListBucketPathReply, error) {
	log.Debugf("received list bucket path request")

	dev, _ := c.DevFromContext(ctx)
	org, _ := c.OrgFromContext(ctx)
	var owner string
	if org != nil {
		owner = org.Name
	} else {
		owner = dev.Username
	}

	buck, _ := c.BucketFromContext(ctx)
	if buck == nil && req.Path == "" {
		return nil, fmt.Errorf("bucket required")
	}

	req.Path = strings.TrimSuffix(req.Path, "/")
	if req.Path == "" { // List top-level buckets
		bucks, err := s.collections.Buckets.List(ctx, owner)
		if err != nil {
			return nil, err
		}
		items := make([]*pb.ListBucketPathReply_Item, len(bucks))
		for i, buck := range bucks {
			buck.
				items[i], err = s.pathToBucketItem(ctx, path.New(buck.Path), false)
			if err != nil {
				return nil, err
			}
			items[i].Name = buck.Name
		}

		return &pb.ListBucketPathReply{
			Item: &pb.ListBucketPathReply_Item{
				IsDir: true,
				Items: items,
			},
		}, nil
	}

	buck, pth, err := s.getBucketAndPathWithScope(ctx, req.Path, scope)
	if err != nil {
		return nil, err
	}

	return s.bucketPathToPb(ctx, buck, pth, true)
}

func (s *service) pathToBucketItem(ctx context.Context, pth path.Path, followLinks bool) (*pb.ListBucketPathReply_Item, error) {
	node, err := s.ipfsClient.Unixfs().Get(ctx, pth)
	if err != nil {
		return nil, err
	}
	defer node.Close()

	return nodeToBucketItem(pth.String(), node, followLinks)
}

func nodeToBucketItem(pth string, node ipfsfiles.Node, followLinks bool) (*pb.ListBucketPathReply_Item, error) {
	size, err := node.Size()
	if err != nil {
		return nil, err
	}
	item := &pb.ListBucketPathReply_Item{
		Name: filepath.Base(pth),
		Path: pth,
		Size: size,
	}
	switch node := node.(type) {
	case ipfsfiles.Directory:
		item.IsDir = true
		entries := node.Entries()
		for entries.Next() {
			if entries.Name() == bucketSeedName {
				continue
			}
			i := &pb.ListBucketPathReply_Item{}
			if followLinks {
				n := entries.Node()
				i, err = nodeToBucketItem(filepath.Join(pth, entries.Name()), n, false)
				if err != nil {
					n.Close()
					return nil, err
				}
				n.Close()
			}
			item.Items = append(item.Items, i)
		}
		if err := entries.Err(); err != nil {
			return nil, err
		}
	}
	return item, nil
}

func (s *service) getBucketAndPathWithScope(ctx context.Context, pth, scope string) (*c.Bucket, path.Path, error) {
	buckName, fileName, err := parsePath(pth)
	if err != nil {
		return nil, nil, err
	}
	buck, err := s.getBucketWithScope(ctx, buckName, scope)
	if err != nil {
		return nil, nil, err
	}
	npth := path.New(filepath.Join(buck.Path, fileName))
	if err = npth.IsValid(); err != nil {
		return nil, nil, err
	}
	return buck, npth, err
}

func (s *service) getBucketWithScope(ctx context.Context, name, scope string) (*c.Bucket, error) {
	buck, err := s.collections.Buckets.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if buck == nil {
		return nil, status.Error(codes.NotFound, "Bucket not found")
	}
	if scope != "*" {
		if _, err := s.getProjectForScope(ctx, buck.ProjectID, scope); err != nil {
			return nil, err
		}
	}
	return buck, nil
}

func parsePath(pth string) (buck, name string, err error) {
	if strings.Contains(pth, bucketSeedName) {
		err = fmt.Errorf("paths containing %s are not allowed", bucketSeedName)
		return
	}
	pth = strings.TrimPrefix(pth, "/")
	parts := strings.SplitN(pth, "/", 2)
	buck = parts[0]
	if len(parts) > 1 {
		name = parts[1]
	}
	return
}

func (s *service) bucketPathToPb(
	ctx context.Context,
	buck *c.Bucket,
	pth path.Path,
	followLinks bool,
) (*pb.ListBucketPathReply, error) {
	item, err := s.pathToBucketItem(ctx, pth, followLinks)
	if err != nil {
		return nil, err
	}

	return &pb.ListBucketPathReply{
		Item: item,
		Root: &pb.BucketRoot{
			Name:    buck.Name,
			Path:    buck.Path,
			Created: buck.Created,
			Updated: buck.Updated,
		},
	}, nil
}

// PushBucketPath handles a push bucket path request.
func (s *service) PushBucketPath(server pb.API_PushBucketPathServer) error {
	log.Debugf("received push bucket path request")

	scope, ok := server.Context().Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}

	req, err := server.Recv()
	if err != nil {
		return err
	}
	var project, filePath string
	switch payload := req.Payload.(type) {
	case *pb.PushBucketPathRequest_Header_:
		project = payload.Header.Project
		filePath = payload.Header.Path
	default:
		return fmt.Errorf("push bucket path header is required")
	}
	buckName, fileName, err := parsePath(filePath)
	if err != nil {
		return err
	}
	buck, err := s.getBucketWithScope(server.Context(), buckName, scope)
	if err != nil {
		if status.Convert(err).Code() != codes.NotFound {
			return err
		}
		proj, err := s.getProjectForScopeByName(server.Context(), project, scope)
		if err != nil {
			return err
		}
		buck, err = s.createBucket(server.Context(), proj, buckName)
		if err != nil {
			return err
		}
	}

	sendEvent := func(event *pb.PushBucketPathReply_Event) error {
		return server.Send(&pb.PushBucketPathReply{
			Payload: &pb.PushBucketPathReply_Event_{
				Event: event,
			},
		})
	}

	sendErr := func(err error) {
		if err2 := server.Send(&pb.PushBucketPathReply{
			Payload: &pb.PushBucketPathReply_Error{
				Error: err.Error(),
			},
		}); err2 != nil {
			log.Errorf("error sending error: %v (%v)", err, err2)
		}
	}

	reader, writer := io.Pipe()
	waitCh := make(chan struct{})
	go func() {
		defer close(waitCh)
		for {
			req, err := server.Recv()
			if err == io.EOF {
				_ = writer.Close()
				return
			} else if err != nil {
				sendErr(fmt.Errorf("error on receive: %v", err))
				_ = writer.CloseWithError(err)
				return
			}
			switch payload := req.Payload.(type) {
			case *pb.PushBucketPathRequest_Chunk:
				if _, err := writer.Write(payload.Chunk); err != nil {
					sendErr(fmt.Errorf("error writing chunk: %v", err))
					return
				}
			default:
				sendErr(fmt.Errorf("invalid request"))
				return
			}
		}
	}()

	var size string
	eventCh := make(chan interface{})
	defer close(eventCh)
	go func() {
		for e := range eventCh {
			event, ok := e.(*iface.AddEvent)
			if !ok {
				log.Error("unexpected event type")
				continue
			}
			if event.Path == nil { // This is a progress event
				if err := sendEvent(&pb.PushBucketPathReply_Event{
					Name:  event.Name,
					Bytes: event.Bytes,
				}); err != nil {
					log.Errorf("error sending event: %v", err)
				}
			} else {
				size = event.Size // Save size for use in the final response
			}
		}
	}()

	pth, err := s.ipfsClient.Unixfs().Add(
		server.Context(),
		ipfsfiles.NewReaderFile(reader),
		options.Unixfs.Pin(false),
		options.Unixfs.Progress(true),
		options.Unixfs.Events(eventCh))
	if err != nil {
		return err
	}

	buckPath := path.New(buck.Path)
	dirpth, err := s.ipfsClient.Object().
		AddLink(server.Context(), buckPath, fileName, pth, options.Object.Create(true))
	if err != nil {
		return err
	}
	if err = s.ipfsClient.Pin().Update(server.Context(), buckPath, dirpth); err != nil {
		return err
	}

	buck.Path = dirpth.String()
	buck.Updated = time.Now().Unix()
	if err = s.collections.Buckets.Save(server.Context(), buck); err != nil {
		return err
	}

	if err = sendEvent(&pb.PushBucketPathReply_Event{
		Path: pth.String(),
		Size: size,
		Root: &pb.BucketRoot{
			Name:    buck.Name,
			Path:    buck.Path,
			Created: buck.Created,
			Updated: buck.Updated,
			Public:  false,
		},
	}); err != nil {
		return err
	}

	log.Debugf("pushed %s to bucket: %s", fileName, buck.Name)
	return nil
}

func (s *service) createBucket(ctx context.Context, proj *c.Project, name string) (*c.Bucket, error) {
	seed := make([]byte, 32)
	_, err := rand.Read(seed)
	if err != nil {
		return nil, err
	}

	pth, err := s.ipfsClient.Unixfs().Add(
		ctx,
		ipfsfiles.NewMapDirectory(map[string]ipfsfiles.Node{
			bucketSeedName: ipfsfiles.NewBytesFile(seed),
		}),
		options.Unixfs.Pin(true))
	if err != nil {
		return nil, err
	}

	if s.dnsManager != nil {
		parts := strings.SplitN(s.gatewayUrl, "//", 2)
		if len(parts) > 1 {
			if _, err := s.dnsManager.NewCNAME(name, parts[1]); err != nil {
				return nil, err
			}
		}
	}

	return s.collections.Buckets.Create(ctx, pth, name, proj.ID)
}

// PullBucketPath handles a pull bucket path request.
func (s *service) PullBucketPath(req *pb.PullBucketPathRequest, server pb.API_PullBucketPathServer) error {
	log.Debugf("received pull bucket path request")

	scope, ok := server.Context().Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}
	_, pth, err := s.getBucketAndPathWithScope(server.Context(), req.Path, scope)
	if err != nil {
		return err
	}
	node, err := s.ipfsClient.Unixfs().Get(server.Context(), pth)
	if err != nil {
		return err
	}
	defer node.Close()

	file := ipfsfiles.ToFile(node)
	if file == nil {
		return fmt.Errorf("node is a directory")
	}
	buf := make([]byte, chunkSize)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			if err := server.Send(&pb.PullBucketPathReply{
				Chunk: buf[:n],
			}); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
	}
	return nil
}

// RemoveBucketPath handles a remove bucket path request.
func (s *service) RemoveBucketPath(
	ctx context.Context,
	req *pb.RemoveBucketPathRequest,
) (*pb.RemoveBucketPathReply, error) {
	log.Debugf("received remove bucket path request")

	scope, ok := ctx.Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}
	buckName, fileName, err := parsePath(req.Path)
	if err != nil {
		return nil, err
	}
	buck, err := s.getBucketWithScope(ctx, buckName, scope)
	if err != nil {
		return nil, err
	}
	buckPath := path.New(buck.Path)

	var dirpth path.Resolved
	var linkCnt int
	if fileName != "" {
		dirpth, err = s.ipfsClient.Object().RmLink(ctx, buckPath, fileName)
		if err != nil {
			return nil, err
		}

		links, err := s.ipfsClient.Unixfs().Ls(ctx, dirpth)
		if err != nil {
			return nil, err
		}
		for range links {
			linkCnt++
		}
	}

	if linkCnt > 1 { // Account for the seed file
		if err = s.ipfsClient.Pin().Update(ctx, buckPath, dirpth); err != nil {
			return nil, err
		}

		buck.Path = dirpth.String()
		buck.Updated = time.Now().Unix()
		if err = s.collections.Buckets.Save(ctx, buck); err != nil {
			return nil, err
		}
		log.Debugf("removed %s from bucket: %s", fileName, buck.Name)
	} else {
		if err = s.ipfsClient.Pin().Rm(ctx, buckPath); err != nil {
			return nil, err
		}

		if err = s.collections.Buckets.Delete(ctx, buck.ID); err != nil {
			return nil, err
		}
		log.Debugf("removed bucket: %s", buck.Name)
	}

	return &pb.RemoveBucketPathReply{}, nil
}

// AddBucketToken handles an add token request.
func (s *service) AddBucketToken(ctx context.Context, req *pb.AddBucketTokenRequest) (*pb.AddBucketTokenReply, error) {
	log.Debugf("received add bucket token request")

	scope, ok := ctx.Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}
	proj, err := s.getProjectForScopeByName(ctx, req.Project, scope)
	if err != nil {
		return nil, err
	}
	token, err := s.collections.BucketTokens.Create(ctx, proj.ID)
	if err != nil {
		return nil, err
	}

	return &pb.AddBucketTokenReply{
		ID: token.ID,
	}, nil
}

// ListBucketTokens handles a list tokens request.
func (s *service) ListBucketTokens(ctx context.Context, req *pb.ListBucketTokensRequest) (*pb.ListBucketTokensReply, error) {
	log.Debugf("received list bucket tokens request")

	scope, ok := ctx.Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}
	proj, err := s.getProjectForScopeByName(ctx, req.Project, scope)
	if err != nil {
		return nil, err
	}

	tokens, err := s.collections.BucketTokens.List(ctx, proj.ID)
	if err != nil {
		return nil, err
	}
	list := make([]string, len(tokens))
	for i, token := range tokens {
		list[i] = token.ID
	}

	return &pb.ListBucketTokensReply{List: list}, nil
}

// RemoveBucketToken handles a remove token request.
func (s *service) RemoveBucketToken(ctx context.Context, req *pb.RemoveBucketTokenRequest) (*pb.RemoveBucketTokenReply, error) {
	log.Debugf("received remove bucket token request")

	scope, ok := ctx.Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}
	token, err := s.getBucketTokenWithScope(ctx, req.ID, scope)
	if err != nil {
		return nil, err
	}

	if err = s.collections.BucketTokens.Delete(ctx, token.ID); err != nil {
		return nil, err
	}

	return &pb.RemoveBucketTokenReply{}, nil
}

func (s *service) getBucketTokenWithScope(ctx context.Context, tokenID, scope string) (*c.BucketToken, error) {
	token, err := s.collections.BucketTokens.Get(ctx, tokenID)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, status.Error(codes.NotFound, "Bucket token not found")
	}
	if _, err := s.getProjectForScope(ctx, token.ProjectID, scope); err != nil {
		return nil, err
	}
	return token, nil
}
