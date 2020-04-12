package cloud

import (
	"context"
	"fmt"
	"net/mail"
	"time"

	logging "github.com/ipfs/go-log"
	threads "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/broadcast"
	"github.com/textileio/go-threads/core/thread"
	pb "github.com/textileio/textile/api/cloud/pb"
	"github.com/textileio/textile/api/common"
	c "github.com/textileio/textile/collections"
	"github.com/textileio/textile/email"
	"github.com/textileio/textile/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	log = logging.Logger("cloud")

	loginTimeout = time.Minute * 3
	emailTimeout = time.Second * 10
)

type Service struct {
	Collections *c.Collections
	Threads     *threads.Client

	EmailClient *email.Client

	GatewayUrl string

	SessionBus    *broadcast.Broadcaster
	SessionSecret string
}

func (s *Service) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginReply, error) {
	log.Debugf("received login request")

	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Email address in not valid")
	}
	dev, err := s.Collections.Developers.GetOrCreate(ctx, req.Username, req.Email)
	if err != nil {
		return nil, err
	}

	secret := getSessionSecret(s.SessionSecret)
	ectx, cancel := context.WithTimeout(ctx, emailTimeout)
	defer cancel()
	if err = s.EmailClient.ConfirmAddress(ectx, dev.Email, s.GatewayUrl, secret); err != nil {
		return nil, err
	}
	if !s.awaitVerification(secret) {
		return nil, status.Error(codes.Unauthenticated, "Could not verify email address")
	}

	session, err := s.Collections.Sessions.Create(ctx, dev.ID)
	if err != nil {
		return nil, err
	}

	if dev.ThreadToken == "" {
		ctx = common.NewSessionContext(ctx, session.Token) // TODO add to credentials
		tok, err := s.Threads.GetToken(ctx, thread.NewLibp2pIdentity(dev.ThreadIdentity))
		if err != nil {
			return nil, err
		}
		if err := s.Collections.Developers.SetDBToken(ctx, dev.ID, tok); err != nil {
			return nil, err
		}
	}

	return &pb.LoginReply{
		ID:       dev.ID.Hex(),
		Username: dev.Username,
		Session:  session.Token,
	}, nil
}

// awaitVerification waits for a dev to verify their email via a sent email.
func (s *Service) awaitVerification(secret string) bool {
	listen := s.SessionBus.Listen()
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

func getSessionSecret(secret string) string {
	if secret != "" {
		return secret
	}
	return util.MakeURLSafeToken(32)
}

func (s *Service) Logout(ctx context.Context, _ *pb.LogoutRequest) (*pb.LogoutReply, error) {
	log.Debugf("received logout request")

	session, _ := c.SessionFromContext(ctx)
	if err := s.Collections.Sessions.Delete(ctx, session.ID); err != nil {
		return nil, err
	}
	return &pb.LogoutReply{}, nil
}

func (s *Service) Whoami(ctx context.Context, _ *pb.WhoamiRequest) (*pb.WhoamiReply, error) {
	log.Debugf("received whoami request")

	dev, _ := c.DevFromContext(ctx)
	return &pb.WhoamiReply{
		ID:       dev.ID.Hex(),
		Username: dev.Username,
		Email:    dev.Email,
	}, nil
}

func (s *Service) GetPrimaryThread(ctx context.Context, _ *pb.GetPrimaryThreadRequest) (*pb.GetPrimaryThreadReply, error) {
	log.Debugf("received get primary thread request")

	dev, _ := c.DevFromContext(ctx)
	doc, err := s.Collections.Threads.GetPrimary(ctx, dev.ID)
	if err != nil {
		return nil, err
	}
	return &pb.GetPrimaryThreadReply{
		ID: doc.ThreadID.Bytes(),
	}, nil
}

func (s *Service) SetPrimaryThread(ctx context.Context, _ *pb.SetPrimaryThreadRequest) (*pb.SetPrimaryThreadReply, error) {
	log.Debugf("received set primary thread request")

	id, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("thread ID required")
	}
	dev, _ := c.DevFromContext(ctx)

	if err := s.Collections.Threads.SetPrimary(ctx, id, dev.ID); err != nil {
		return nil, err
	}
	return &pb.SetPrimaryThreadReply{}, nil
}

func (s *Service) ListThreads(ctx context.Context, _ *pb.ListThreadsRequest) (*pb.ListThreadsReply, error) {
	log.Debugf("received list threads request")

	dev, _ := c.DevFromContext(ctx)
	list, err := s.Collections.Threads.List(ctx, dev.ID)
	if err != nil {
		return nil, err
	}
	reply := &pb.ListThreadsReply{
		List: make([]*pb.ListThreadsReply_Thread, len(list)),
	}
	for i, t := range list {
		reply.List[i] = &pb.ListThreadsReply_Thread{
			ID:      t.ThreadID.Bytes(),
			Primary: t.Primary,
		}
	}
	return reply, nil
}

func (s *Service) AddOrg(ctx context.Context, req *pb.AddOrgRequest) (*pb.GetOrgReply, error) {
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
	if err := s.Collections.Orgs.Create(ctx, org); err != nil {
		return nil, err
	}
	return orgToPbOrg(org), nil
}

func (s *Service) GetOrg(ctx context.Context, _ *pb.GetOrgRequest) (*pb.GetOrgReply, error) {
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

func (s *Service) ListOrgs(ctx context.Context, _ *pb.ListOrgsRequest) (*pb.ListOrgsReply, error) {
	log.Debugf("received list orgs request")

	dev, _ := c.DevFromContext(ctx)
	orgs, err := s.Collections.Orgs.List(ctx, dev.ID)
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
func (s *Service) RemoveOrg(ctx context.Context, _ *pb.RemoveOrgRequest) (*pb.RemoveOrgReply, error) {
	log.Debugf("received remove org request")

	dev, _ := c.DevFromContext(ctx)
	org, ok := c.OrgFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("org required")
	}
	isOwner, err := s.Collections.Orgs.IsOwner(ctx, org.Name, dev.ID)
	if err != nil {
		return nil, err
	}
	if !isOwner {
		return nil, status.Error(codes.PermissionDenied, "User must be an org owner")
	}

	if err = s.Collections.Orgs.Delete(ctx, org.ID); err != nil {
		return nil, err
	}
	return &pb.RemoveOrgReply{}, nil
}

func (s *Service) InviteToOrg(ctx context.Context, req *pb.InviteToOrgRequest) (*pb.InviteToOrgReply, error) {
	log.Debugf("received invite to org request")

	dev, _ := c.DevFromContext(ctx)
	org, ok := c.OrgFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("org required")
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Email address in not valid")
	}
	invite, err := s.Collections.Invites.Create(ctx, dev.ID, org.Name, req.Email)
	if err != nil {
		return nil, err
	}

	ectx, cancel := context.WithTimeout(ctx, emailTimeout)
	defer cancel()
	if err = s.EmailClient.InviteAddress(
		ectx, org.Name, dev.Email, req.Email, s.GatewayUrl, invite.Token); err != nil {
		return nil, err
	}
	return &pb.InviteToOrgReply{Token: invite.Token}, nil
}

func (s *Service) LeaveOrg(ctx context.Context, _ *pb.LeaveOrgRequest) (*pb.LeaveOrgReply, error) {
	log.Debugf("received leave org request")

	dev, _ := c.DevFromContext(ctx)
	org, ok := c.OrgFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("org required")
	}
	if err := s.Collections.Orgs.RemoveMember(ctx, org.Name, dev.ID); err != nil {
		return nil, err
	}
	return &pb.LeaveOrgReply{}, nil
}
