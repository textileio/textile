package hub

import (
	"context"
	"fmt"
	"net/mail"
	"time"

	logging "github.com/ipfs/go-log"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/crypto"
	threads "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/broadcast"
	net "github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	netclient "github.com/textileio/go-threads/net/api/client"
	"github.com/textileio/textile/api/common"
	pb "github.com/textileio/textile/api/hub/pb"
	"github.com/textileio/textile/buckets"
	bc "github.com/textileio/textile/buckets/collection"
	c "github.com/textileio/textile/collections"
	"github.com/textileio/textile/dns"
	"github.com/textileio/textile/email"
	"github.com/textileio/textile/ipns"
	"github.com/textileio/textile/util"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	log = logging.Logger("hubapi")

	loginTimeout = time.Minute * 3
	emailTimeout = time.Second * 10
)

type Service struct {
	Collections        *c.Collections
	Threads            *threads.Client
	ThreadsNet         *netclient.Client
	GatewayURL         string
	EmailClient        *email.Client
	EmailSessionBus    *broadcast.Broadcaster
	EmailSessionSecret string
	IPFSClient         iface.CoreAPI
	IPNSManager        *ipns.Manager
	DNSManager         *dns.Manager
}

func (s *Service) Signup(ctx context.Context, req *pb.SignupRequest) (*pb.SignupReply, error) {
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

	dev, err := s.Collections.Accounts.CreateDev(ctx, req.Username, req.Email)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Account exists")
	}
	session, err := s.Collections.Sessions.Create(ctx, dev.Key)
	if err != nil {
		return nil, err
	}
	ctx = common.NewSessionContext(ctx, session.ID)
	tok, err := s.Threads.GetToken(ctx, thread.NewLibp2pIdentity(dev.Secret))
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
			if err := s.Collections.Accounts.AddMember(ctx, invite.Org, c.Member{
				Key:      dev.Key,
				Username: dev.Username,
				Role:     c.OrgMember,
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

	key, err := crypto.MarshalPublicKey(dev.Key)
	if err != nil {
		return nil, err
	}
	return &pb.SignupReply{
		Key:     key,
		Session: session.ID,
	}, nil
}

func (s *Service) Signin(ctx context.Context, req *pb.SigninRequest) (*pb.SigninReply, error) {
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

	key, err := crypto.MarshalPublicKey(dev.Key)
	if err != nil {
		return nil, err
	}
	return &pb.SigninReply{
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

func (s *Service) Signout(ctx context.Context, _ *pb.SignoutRequest) (*pb.SignoutReply, error) {
	log.Debugf("received signout request")

	session, _ := c.SessionFromContext(ctx)
	if err := s.Collections.Sessions.Delete(ctx, session.ID); err != nil {
		return nil, err
	}
	return &pb.SignoutReply{}, nil
}

func (s *Service) GetSessionInfo(ctx context.Context, _ *pb.GetSessionInfoRequest) (*pb.GetSessionInfoReply, error) {
	log.Debugf("received get session info request")

	dev, _ := c.DevFromContext(ctx)
	key, err := crypto.MarshalPublicKey(dev.Key)
	if err != nil {
		return nil, err
	}
	return &pb.GetSessionInfoReply{
		Key:      key,
		Username: dev.Username,
		Email:    dev.Email,
	}, nil
}

func (s *Service) CreateKey(ctx context.Context, req *pb.CreateKeyRequest) (*pb.GetKeyReply, error) {
	log.Debugf("received create key request")

	owner := ownerFromContext(ctx)
	key, err := s.Collections.APIKeys.Create(ctx, owner, c.APIKeyType(req.Type), req.Secure)
	if err != nil {
		return nil, err
	}
	return &pb.GetKeyReply{
		Key:     key.Key,
		Secret:  key.Secret,
		Type:    pb.KeyType(key.Type),
		Valid:   true,
		Threads: 0,
		Secure:  key.Secure,
	}, nil
}

func (s *Service) InvalidateKey(ctx context.Context, req *pb.InvalidateKeyRequest) (*pb.InvalidateKeyReply, error) {
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
	return &pb.InvalidateKeyReply{}, nil
}

func (s *Service) ListKeys(ctx context.Context, _ *pb.ListKeysRequest) (*pb.ListKeysReply, error) {
	log.Debugf("received list keys request")

	owner := ownerFromContext(ctx)
	keys, err := s.Collections.APIKeys.ListByOwner(ctx, owner)
	if err != nil {
		return nil, err
	}
	list := make([]*pb.GetKeyReply, len(keys))
	for i, key := range keys {
		ts, err := s.Collections.Threads.ListByKey(ctx, key.Key)
		if err != nil {
			return nil, err
		}
		list[i] = &pb.GetKeyReply{
			Key:     key.Key,
			Secret:  key.Secret,
			Type:    pb.KeyType(key.Type),
			Valid:   key.Valid,
			Threads: int32(len(ts)),
			Secure:  key.Secure,
		}
	}
	return &pb.ListKeysReply{List: list}, nil
}

func (s *Service) CreateOrg(ctx context.Context, req *pb.CreateOrgRequest) (*pb.GetOrgReply, error) {
	log.Debugf("received create org request")

	dev, _ := c.DevFromContext(ctx)
	org, err := s.Collections.Accounts.CreateOrg(ctx, req.Name, []c.Member{{
		Key:      dev.Key,
		Username: dev.Username,
		Role:     c.OrgOwner,
	}})
	if err != nil {
		return nil, err
	}
	tok, err := s.Threads.GetToken(ctx, thread.NewLibp2pIdentity(org.Secret))
	if err != nil {
		return nil, err
	}
	if err := s.Collections.Accounts.SetToken(ctx, org.Key, tok); err != nil {
		return nil, err
	}
	return s.orgToPbOrg(org)
}

func (s *Service) GetOrg(ctx context.Context, _ *pb.GetOrgRequest) (*pb.GetOrgReply, error) {
	log.Debugf("received get org request")

	org, ok := c.OrgFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("org required")
	}
	return s.orgToPbOrg(org)
}

func (s *Service) orgToPbOrg(org *c.Account) (*pb.GetOrgReply, error) {
	members := make([]*pb.GetOrgReply_Member, len(org.Members))
	for i, m := range org.Members {
		key, err := crypto.MarshalPublicKey(m.Key)
		if err != nil {
			return nil, err
		}
		members[i] = &pb.GetOrgReply_Member{
			Key:      key,
			Username: m.Username,
			Role:     m.Role.String(),
		}
	}
	key, err := crypto.MarshalPublicKey(org.Key)
	if err != nil {
		return nil, err
	}
	return &pb.GetOrgReply{
		Key:       key,
		Name:      org.Name,
		Slug:      org.Username,
		Host:      s.GatewayURL,
		Members:   members,
		CreatedAt: org.CreatedAt.Unix(),
	}, nil
}

func (s *Service) ListOrgs(ctx context.Context, _ *pb.ListOrgsRequest) (*pb.ListOrgsReply, error) {
	log.Debugf("received list orgs request")

	dev, _ := c.DevFromContext(ctx)
	orgs, err := s.Collections.Accounts.ListByMember(ctx, dev.Key)
	if err != nil {
		return nil, err
	}
	list := make([]*pb.GetOrgReply, len(orgs))
	for i, org := range orgs {
		list[i], err = s.orgToPbOrg(&org)
		if err != nil {
			return nil, err
		}
	}
	return &pb.ListOrgsReply{List: list}, nil
}

func (s *Service) RemoveOrg(ctx context.Context, _ *pb.RemoveOrgRequest) (*pb.RemoveOrgReply, error) {
	log.Debugf("received remove org request")

	dev, _ := c.DevFromContext(ctx)
	org, ok := c.OrgFromContext(ctx)
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
	return &pb.InviteToOrgReply{Token: invite.Token}, nil
}

func (s *Service) LeaveOrg(ctx context.Context, _ *pb.LeaveOrgRequest) (*pb.LeaveOrgReply, error) {
	log.Debugf("received leave org request")

	dev, _ := c.DevFromContext(ctx)
	org, ok := c.OrgFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("org required")
	}
	if err := s.Collections.Accounts.RemoveMember(ctx, org.Username, dev.Key); err != nil {
		return nil, err
	}
	if err := s.Collections.Invites.DeleteByFromAndOrg(ctx, dev.Key, org.Username); err != nil {
		return nil, err
	}
	return &pb.LeaveOrgReply{}, nil
}

func (s *Service) IsUsernameAvailable(ctx context.Context, req *pb.IsUsernameAvailableRequest) (*pb.IsUsernameAvailableReply, error) {
	log.Debugf("received is username available request")

	if err := s.Collections.Accounts.IsUsernameAvailable(ctx, req.Username); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	return &pb.IsUsernameAvailableReply{}, nil
}

func (s *Service) IsOrgNameAvailable(ctx context.Context, req *pb.IsOrgNameAvailableRequest) (*pb.IsOrgNameAvailableReply, error) {
	log.Debugf("received is org name available request")

	slug, err := s.Collections.Accounts.IsNameAvailable(ctx, req.Name)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	return &pb.IsOrgNameAvailableReply{
		Slug: slug,
		Host: s.GatewayURL,
	}, nil
}

func (s *Service) DestroyAccount(ctx context.Context, _ *pb.DestroyAccountRequest) (*pb.DestroyAccountReply, error) {
	log.Debugf("received destroy account request")

	dev, _ := c.DevFromContext(ctx)
	if err := s.destroyAccount(ctx, dev); err != nil {
		return nil, err
	}
	return &pb.DestroyAccountReply{}, nil
}

func ownerFromContext(ctx context.Context) crypto.PubKey {
	org, ok := c.OrgFromContext(ctx)
	if !ok {
		dev, _ := c.DevFromContext(ctx)
		return dev.Key
	}
	return org.Key
}

func (s *Service) destroyAccount(ctx context.Context, a *c.Account) error {
	// First, ensure that the account does not own any orgs
	if a.Type == c.Dev {
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
			bres, err := s.Threads.Find(ctx, t.ID, buckets.CollectionName, &db.Query{}, &bc.Bucket{}, db.WithTxnToken(a.Token))
			if err != nil {
				return err
			}
			for _, b := range bres.([]*bc.Bucket) {
				if err = s.IPFSClient.Pin().Rm(ctx, path.New(b.Path)); err != nil {
					return err
				}
				if err = s.IPNSManager.RemoveKey(ctx, b.Key); err != nil {
					return err
				}
				if b.DNSRecord != "" && s.DNSManager != nil {
					if err = s.DNSManager.DeleteRecord(b.DNSRecord); err != nil {
						return err
					}
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
	if a.Type == c.Org {
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
