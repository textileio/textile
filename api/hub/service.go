package hub

import (
	"context"
	"fmt"
	"net/mail"
	"time"

	logging "github.com/ipfs/go-log"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/path"
	threads "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/broadcast"
	net "github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	netclient "github.com/textileio/go-threads/net/api/client"
	powc "github.com/textileio/powergate/api/client"
	"github.com/textileio/textile/api/common"
	pb "github.com/textileio/textile/api/hub/pb"
	"github.com/textileio/textile/buckets"
	bi "github.com/textileio/textile/buildinfo"
	"github.com/textileio/textile/dns"
	"github.com/textileio/textile/email"
	"github.com/textileio/textile/ipns"
	mdb "github.com/textileio/textile/mongodb"
	tdb "github.com/textileio/textile/threaddb"
	"github.com/textileio/textile/util"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	log = logging.Logger("hubapi")

	loginTimeout = time.Minute * 30
	emailTimeout = time.Second * 10
)

type Service struct {
	Collections        *mdb.Collections
	Threads            *threads.Client
	ThreadsNet         *netclient.Client
	GatewayURL         string
	EmailClient        *email.Client
	EmailSessionBus    *broadcast.Broadcaster
	EmailSessionSecret string
	IPFSClient         iface.CoreAPI
	IPNSManager        *ipns.Manager
	DNSManager         *dns.Manager
	Pow                *powc.Client
}

// Info provides the currently running API's build information
func (s *Service) BuildInfo(ctx context.Context, _ *pb.BuildInfoRequest) (*pb.BuildInfoResponse, error) {
	return &pb.BuildInfoResponse{
		GitCommit:  bi.GitCommit,
		GitBranch:  bi.GitBranch,
		GitState:   bi.GitState,
		GitSummary: bi.GitSummary,
		BuildDate:  bi.BuildDate,
		Version:    bi.Version,
	}, nil
}

func (s *Service) Signup(ctx context.Context, req *pb.SignupRequest) (*pb.SignupResponse, error) {
	log.Debugf("received signup request")

	if err := s.Collections.Accounts.ValidateUsername(req.Username); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Email address in not valid")
	}

	secret := getSessionSecret(s.EmailSessionSecret)
	ectx, cancel := context.WithTimeout(ctx, emailTimeout)
	defer cancel()
	if err := s.EmailClient.ConfirmAddress(ectx, req.Email, s.GatewayURL, secret); err != nil {
		return nil, err
	}
	if !s.awaitVerification(secret) {
		return nil, status.Error(codes.Unauthenticated, "Could not verify email address")
	}

	var powInfo *mdb.PowInfo
	if s.Pow != nil {
		ffsId, ffsToken, err := s.Pow.FFS.Create(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Unable to create FFS instance: %v", err)
		}
		powInfo = &mdb.PowInfo{ID: ffsId, Token: ffsToken}
	}

	dev, err := s.Collections.Accounts.CreateDev(ctx, req.Username, req.Email, powInfo)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Account exists")
	}
	session, err := s.Collections.Sessions.Create(ctx, dev.Key)
	if err != nil {
		return nil, err
	}
	ctx = common.NewSessionContext(ctx, session.ID)
	tok, err := s.Threads.GetToken(ctx, dev.Secret)
	if err != nil {
		return nil, err
	}
	if err := s.Collections.Accounts.SetToken(ctx, dev.Key, tok); err != nil {
		return nil, err
	}

	// Check for pending invites
	invites, err := s.Collections.Invites.ListByEmail(ctx, dev.Email)
	if err != nil {
		return nil, err
	}
	for _, invite := range invites {
		if invite.Accepted {
			if err := s.Collections.Accounts.AddMember(ctx, invite.Org, mdb.Member{
				Key:      dev.Key,
				Username: dev.Username,
				Role:     mdb.OrgMember,
			}); err != nil {
				if err == mongo.ErrNoDocuments {
					if err := s.Collections.Invites.Delete(ctx, invite.Token); err != nil {
						return nil, err
					}
				} else {
					return nil, err
				}
			}
			if err := s.Collections.Invites.Delete(ctx, invite.Token); err != nil {
				return nil, err
			}
		}
		if time.Now().After(invite.ExpiresAt) {
			if err := s.Collections.Invites.Delete(ctx, invite.Token); err != nil {
				return nil, err
			}
		}
	}

	key, err := dev.Key.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return &pb.SignupResponse{
		Key:     key,
		Session: session.ID,
	}, nil
}

func (s *Service) Signin(ctx context.Context, req *pb.SigninRequest) (*pb.SigninResponse, error) {
	log.Debugf("received signin request")

	dev, err := s.Collections.Accounts.GetByUsernameOrEmail(ctx, req.UsernameOrEmail)
	if err != nil {
		return nil, status.Error(codes.NotFound, "User not found")
	}

	secret := getSessionSecret(s.EmailSessionSecret)
	ectx, cancel := context.WithTimeout(ctx, emailTimeout)
	defer cancel()
	if err = s.EmailClient.ConfirmAddress(ectx, dev.Email, s.GatewayURL, secret); err != nil {
		return nil, err
	}
	if !s.awaitVerification(secret) {
		return nil, status.Error(codes.Unauthenticated, "Could not verify email address")
	}

	session, err := s.Collections.Sessions.Create(ctx, dev.Key)
	if err != nil {
		return nil, err
	}

	key, err := dev.Key.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return &pb.SigninResponse{
		Key:     key,
		Session: session.ID,
	}, nil
}

// awaitVerification waits for a dev to verify their email via a sent email.
func (s *Service) awaitVerification(secret string) bool {
	listen := s.EmailSessionBus.Listen()
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

// getSessionSecret returns a random secret for use with email verification.
// To cover tests that need to auto-verify, the API can be started with a static secret.
func getSessionSecret(secret string) string {
	if secret != "" {
		return secret
	}
	return util.MakeToken(44)
}

func (s *Service) Signout(ctx context.Context, _ *pb.SignoutRequest) (*pb.SignoutResponse, error) {
	log.Debugf("received signout request")

	session, _ := mdb.SessionFromContext(ctx)
	if err := s.Collections.Sessions.Delete(ctx, session.ID); err != nil {
		return nil, err
	}
	return &pb.SignoutResponse{}, nil
}

func (s *Service) GetSessionInfo(ctx context.Context, _ *pb.GetSessionInfoRequest) (*pb.GetSessionInfoResponse, error) {
	log.Debugf("received get session info request")

	dev, _ := mdb.DevFromContext(ctx)
	key, err := dev.Key.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return &pb.GetSessionInfoResponse{
		Key:      key,
		Username: dev.Username,
		Email:    dev.Email,
	}, nil
}

func (s *Service) GetIdentity(ctx context.Context, _ *pb.GetIdentityRequest) (*pb.GetIdentityResponse, error) {
	log.Debugf("received get identity request")

	var identity thread.Identity
	org, ok := mdb.OrgFromContext(ctx)
	if !ok {
		dev, _ := mdb.DevFromContext(ctx)
		identity = dev.Secret
	} else {
		identity = org.Secret
	}
	data, err := identity.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return &pb.GetIdentityResponse{
		Identity: data,
	}, nil
}

func (s *Service) CreateKey(ctx context.Context, req *pb.CreateKeyRequest) (*pb.CreateKeyResponse, error) {
	log.Debugf("received create key request")

	owner := ownerFromContext(ctx)

	var keyType mdb.APIKeyType
	switch req.Type {
	case pb.KeyType_KEY_TYPE_ACCOUNT:
	case pb.KeyType_KEY_TYPE_UNSPECIFIED:
		keyType = mdb.AccountKey
	case pb.KeyType_KEY_TYPE_USER:
		keyType = mdb.UserKey
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid key type: %v", req.Type.String())
	}

	key, err := s.Collections.APIKeys.Create(ctx, owner, keyType, req.Secure)
	if err != nil {
		return nil, err
	}
	t, err := keyTypeToPb(key.Type)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mapping key type: %v", key.Type)
	}
	return &pb.CreateKeyResponse{
		KeyInfo: &pb.KeyInfo{
			Key:     key.Key,
			Secret:  key.Secret,
			Type:    t,
			Valid:   true,
			Threads: 0,
			Secure:  key.Secure,
		},
	}, nil
}

func (s *Service) InvalidateKey(ctx context.Context, req *pb.InvalidateKeyRequest) (*pb.InvalidateKeyResponse, error) {
	log.Debugf("received invalidate key request")

	key, err := s.Collections.APIKeys.Get(ctx, req.Key)
	if err != nil {
		return nil, err
	}
	owner := ownerFromContext(ctx)
	if !owner.Equals(key.Owner) {
		return nil, status.Error(codes.PermissionDenied, "User does not own key")
	}
	if err := s.Collections.APIKeys.Invalidate(ctx, req.Key); err != nil {
		return nil, err
	}
	return &pb.InvalidateKeyResponse{}, nil
}

func (s *Service) ListKeys(ctx context.Context, _ *pb.ListKeysRequest) (*pb.ListKeysResponse, error) {
	log.Debugf("received list keys request")

	owner := ownerFromContext(ctx)
	keys, err := s.Collections.APIKeys.ListByOwner(ctx, owner)
	if err != nil {
		return nil, err
	}
	list := make([]*pb.KeyInfo, len(keys))
	for i, key := range keys {
		t, err := keyTypeToPb(key.Type)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "mapping key type: %v", key.Type)
		}
		ts, err := s.Collections.Threads.ListByKey(ctx, key.Key)
		if err != nil {
			return nil, err
		}
		list[i] = &pb.KeyInfo{
			Key:     key.Key,
			Secret:  key.Secret,
			Type:    t,
			Valid:   key.Valid,
			Threads: int32(len(ts)),
			Secure:  key.Secure,
		}
	}
	return &pb.ListKeysResponse{List: list}, nil
}

func (s *Service) CreateOrg(ctx context.Context, req *pb.CreateOrgRequest) (*pb.CreateOrgResponse, error) {
	log.Debugf("received create org request")

	dev, _ := mdb.DevFromContext(ctx)

	var powInfo *mdb.PowInfo
	if s.Pow != nil {
		ffsId, ffsToken, err := s.Pow.FFS.Create(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Unable to create FFS instance: %v", err)
		}
		powInfo = &mdb.PowInfo{ID: ffsId, Token: ffsToken}
	}
	org, err := s.Collections.Accounts.CreateOrg(ctx, req.Name, []mdb.Member{{
		Key:      dev.Key,
		Username: dev.Username,
		Role:     mdb.OrgOwner,
	}}, powInfo)
	if err != nil {
		return nil, err
	}
	tok, err := s.Threads.GetToken(ctx, org.Secret)
	if err != nil {
		return nil, err
	}
	if err := s.Collections.Accounts.SetToken(ctx, org.Key, tok); err != nil {
		return nil, err
	}
	orgInfo, err := s.orgToPbOrg(org)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to encode OrgInfo: %v", err)
	}
	return &pb.CreateOrgResponse{
		OrgInfo: orgInfo,
	}, nil
}

func (s *Service) GetOrg(ctx context.Context, _ *pb.GetOrgRequest) (*pb.GetOrgResponse, error) {
	log.Debugf("received get org request")

	org, ok := mdb.OrgFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("org required")
	}
	orgInfo, err := s.orgToPbOrg(org)
	if err != nil {
		return nil, fmt.Errorf("encoding org: %v", err)
	}
	return &pb.GetOrgResponse{
		OrgInfo: orgInfo,
	}, nil
}

func (s *Service) orgToPbOrg(org *mdb.Account) (*pb.OrgInfo, error) {
	members := make([]*pb.OrgInfo_Member, len(org.Members))
	for i, m := range org.Members {
		key, err := m.Key.MarshalBinary()
		if err != nil {
			return nil, err
		}
		members[i] = &pb.OrgInfo_Member{
			Key:      key,
			Username: m.Username,
			Role:     m.Role.String(),
		}
	}
	key, err := org.Key.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return &pb.OrgInfo{
		Key:       key,
		Name:      org.Name,
		Slug:      org.Username,
		Host:      s.GatewayURL,
		Members:   members,
		CreatedAt: org.CreatedAt.Unix(),
	}, nil
}

func (s *Service) ListOrgs(ctx context.Context, _ *pb.ListOrgsRequest) (*pb.ListOrgsResponse, error) {
	log.Debugf("received list orgs request")

	dev, _ := mdb.DevFromContext(ctx)
	orgs, err := s.Collections.Accounts.ListByMember(ctx, dev.Key)
	if err != nil {
		return nil, err
	}
	list := make([]*pb.OrgInfo, len(orgs))
	for i, org := range orgs {
		list[i], err = s.orgToPbOrg(&org)
		if err != nil {
			return nil, err
		}
	}
	return &pb.ListOrgsResponse{List: list}, nil
}

func (s *Service) RemoveOrg(ctx context.Context, _ *pb.RemoveOrgRequest) (*pb.RemoveOrgResponse, error) {
	log.Debugf("received remove org request")

	dev, _ := mdb.DevFromContext(ctx)
	org, ok := mdb.OrgFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("org required")
	}
	isOwner, err := s.Collections.Accounts.IsOwner(ctx, org.Username, dev.Key)
	if err != nil {
		return nil, err
	}
	if !isOwner {
		return nil, status.Error(codes.PermissionDenied, "User must be an org owner")
	}

	if err = s.destroyAccount(ctx, org); err != nil {
		return nil, err
	}
	return &pb.RemoveOrgResponse{}, nil
}

func (s *Service) InviteToOrg(ctx context.Context, req *pb.InviteToOrgRequest) (*pb.InviteToOrgResponse, error) {
	log.Debugf("received invite to org request")

	dev, _ := mdb.DevFromContext(ctx)
	org, ok := mdb.OrgFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("org required")
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Email address in not valid")
	}
	invite, err := s.Collections.Invites.Create(ctx, dev.Key, org.Username, req.Email)
	if err != nil {
		return nil, err
	}

	ectx, cancel := context.WithTimeout(ctx, emailTimeout)
	defer cancel()
	if err = s.EmailClient.InviteAddress(
		ectx, org.Name, dev.Email, req.Email, s.GatewayURL, invite.Token); err != nil {
		return nil, err
	}
	return &pb.InviteToOrgResponse{Token: invite.Token}, nil
}

func (s *Service) LeaveOrg(ctx context.Context, _ *pb.LeaveOrgRequest) (*pb.LeaveOrgResponse, error) {
	log.Debugf("received leave org request")

	dev, _ := mdb.DevFromContext(ctx)
	org, ok := mdb.OrgFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("org required")
	}
	if err := s.Collections.Accounts.RemoveMember(ctx, org.Username, dev.Key); err != nil {
		return nil, err
	}
	if err := s.Collections.Invites.DeleteByFromAndOrg(ctx, dev.Key, org.Username); err != nil {
		return nil, err
	}
	return &pb.LeaveOrgResponse{}, nil
}

func (s *Service) IsUsernameAvailable(ctx context.Context, req *pb.IsUsernameAvailableRequest) (*pb.IsUsernameAvailableResponse, error) {
	log.Debugf("received is username available request")

	if err := s.Collections.Accounts.IsUsernameAvailable(ctx, req.Username); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	return &pb.IsUsernameAvailableResponse{}, nil
}

func (s *Service) IsOrgNameAvailable(ctx context.Context, req *pb.IsOrgNameAvailableRequest) (*pb.IsOrgNameAvailableResponse, error) {
	log.Debugf("received is org name available request")

	slug, err := s.Collections.Accounts.IsNameAvailable(ctx, req.Name)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	return &pb.IsOrgNameAvailableResponse{
		Slug: slug,
		Host: s.GatewayURL,
	}, nil
}

func (s *Service) DestroyAccount(ctx context.Context, _ *pb.DestroyAccountRequest) (*pb.DestroyAccountResponse, error) {
	log.Debugf("received destroy account request")

	dev, _ := mdb.DevFromContext(ctx)
	if err := s.destroyAccount(ctx, dev); err != nil {
		return nil, err
	}
	return &pb.DestroyAccountResponse{}, nil
}

func ownerFromContext(ctx context.Context) thread.PubKey {
	org, ok := mdb.OrgFromContext(ctx)
	if !ok {
		dev, _ := mdb.DevFromContext(ctx)
		return dev.Key
	}
	return org.Key
}

func (s *Service) destroyAccount(ctx context.Context, a *mdb.Account) error {
	// First, ensure that the account does not own any orgs
	if a.Type == mdb.Dev {
		orgs, err := s.Collections.Accounts.ListByOwner(ctx, a.Key)
		if err != nil {
			return err
		}
		if len(orgs) > 0 {
			return status.Error(codes.FailedPrecondition, "Account not empty (delete orgs first)")
		}
	}

	// Collect threads owned directly or via an API key
	ts, err := s.Collections.Threads.ListByOwner(ctx, a.Key)
	if err != nil {
		return err
	}
	keys, err := s.Collections.APIKeys.ListByOwner(ctx, a.Key)
	if err != nil {
		return err
	}
	for _, k := range keys {
		kts, err := s.Collections.Threads.ListByKey(ctx, k.Key)
		if err != nil {
			return err
		}
		ts = append(ts, kts...)
	}

	for _, t := range ts {
		if t.IsDB {
			// Clean up bucket pins, keys, and dns records.
			bres, err := s.Threads.Find(ctx, t.ID, buckets.CollectionName, &db.Query{}, &tdb.Bucket{}, db.WithTxnToken(a.Token))
			if err != nil {
				return err
			}
			for _, b := range bres.([]*tdb.Bucket) {
				if err = s.IPFSClient.Pin().Rm(ctx, path.New(b.Path)); err != nil {
					return err
				}
				if err = s.IPNSManager.RemoveKey(ctx, b.Key); err != nil {
					return err
				}
			}
			// Delete the entire DB.
			if err := s.Threads.DeleteDB(ctx, t.ID, db.WithManagedToken(a.Token)); err != nil {
				return err
			}
		} else {
			// Delete the entire thread.
			if err := s.ThreadsNet.DeleteThread(ctx, t.ID, net.WithThreadToken(a.Token)); err != nil {
				return err
			}
		}
	}
	// Stop tracking the deleted threads.
	if err = s.Collections.Threads.DeleteByOwner(ctx, a.Key); err != nil {
		return err
	}

	// Clean up other associated objects.
	if err = s.Collections.APIKeys.DeleteByOwner(ctx, a.Key); err != nil {
		return err
	}
	if err = s.Collections.Sessions.DeleteByOwner(ctx, a.Key); err != nil {
		return err
	}
	if a.Type == mdb.Org {
		if err = s.Collections.Invites.DeleteByOrg(ctx, a.Username); err != nil {
			return err
		}
	} else {
		if err = s.Collections.Invites.DeleteByFrom(ctx, a.Key); err != nil {
			return err
		}
	}

	// Finally, delete the account.
	return s.Collections.Accounts.Delete(ctx, a.Key)
}

func keyTypeToPb(t mdb.APIKeyType) (pb.KeyType, error) {
	switch t {
	case mdb.AccountKey:
		return pb.KeyType_KEY_TYPE_ACCOUNT, nil
	case mdb.UserKey:
		return pb.KeyType_KEY_TYPE_USER, nil
	default:
		return 0, fmt.Errorf("unknown key type: %v", t)
	}
}
