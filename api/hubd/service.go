package hubd

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"time"

	logging "github.com/ipfs/go-log/v2"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/path"
	threads "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/broadcast"
	net "github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/db"
	netclient "github.com/textileio/go-threads/net/api/client"
	pow "github.com/textileio/powergate/v2/api/client"
	"github.com/textileio/textile/v2/api/billingd/analytics"
	billing "github.com/textileio/textile/v2/api/billingd/client"
	"github.com/textileio/textile/v2/api/common"
	pb "github.com/textileio/textile/v2/api/hubd/pb"
	"github.com/textileio/textile/v2/buckets"
	bi "github.com/textileio/textile/v2/buildinfo"
	"github.com/textileio/textile/v2/email"
	"github.com/textileio/textile/v2/ipns"
	mdb "github.com/textileio/textile/v2/mongodb"
	tdb "github.com/textileio/textile/v2/threaddb"
	"github.com/textileio/textile/v2/util"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	log = logging.Logger("hubapi")

	errDevRequired = errors.New("developer account required")
	errOrgRequired = errors.New("organization account required")

	loginTimeout = time.Minute * 30
	emailTimeout = time.Second * 10
)

type Service struct {
	Collections         *mdb.Collections
	Threads             *threads.Client
	ThreadsNet          *netclient.Client
	GatewayURL          string
	EmailClient         *email.Client
	EmailSessionBus     *broadcast.Broadcaster
	EmailSessionSecret  string
	IPFSClient          iface.CoreAPI
	IPNSManager         *ipns.Manager
	BillingClient       *billing.Client
	PowergateClient     *pow.Client
	PowergateAdminToken string
}

// Info provides the currently running API's build information
func (s *Service) BuildInfo(_ context.Context, _ *pb.BuildInfoRequest) (*pb.BuildInfoResponse, error) {
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
	if err := s.EmailClient.ConfirmAddress(ectx, req.Email, req.Username, req.Email, s.GatewayURL, secret); err != nil {
		return nil, err
	}
	if !s.awaitVerification(secret) {
		return nil, status.Error(codes.Unauthenticated, "Could not verify email address")
	}

	var powInfo *mdb.PowInfo
	if s.PowergateClient != nil {
		res, err := s.PowergateClient.Admin.Users.Create(s.powergateAdminCtx(ctx))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Unable to create user: %v", err)
		}
		powInfo = &mdb.PowInfo{ID: res.User.Id, Token: res.User.Token}
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
	if err = s.EmailClient.ConfirmAddress(
		ectx,
		dev.Key.String(),
		dev.Username,
		dev.Email,
		s.GatewayURL,
		secret,
	); err != nil {
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

	if s.BillingClient != nil {
		s.BillingClient.TrackEvent(ctx, dev.Key, mdb.Dev, true, analytics.SignIn, nil)
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

func (s *Service) powergateAdminCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, pow.AdminKey, s.PowergateAdminToken)
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

	session, ok := mdb.SessionFromContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "session not found")
	}
	if err := s.Collections.Sessions.Delete(ctx, session.ID); err != nil {
		return nil, err
	}
	return &pb.SignoutResponse{}, nil
}

func (s *Service) GetSessionInfo(ctx context.Context, _ *pb.GetSessionInfoRequest) (*pb.GetSessionInfoResponse, error) {
	log.Debugf("received get session info request")

	account, err := getAccount(ctx)
	if err != nil {
		return nil, err
	}
	if account.User == nil {
		return nil, status.Errorf(codes.InvalidArgument, errDevRequired.Error())
	}
	key, err := account.User.Key.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return &pb.GetSessionInfoResponse{
		Key:      key,
		Username: account.User.Username,
		Email:    account.User.Email,
	}, nil
}

func (s *Service) GetIdentity(ctx context.Context, _ *pb.GetIdentityRequest) (*pb.GetIdentityResponse, error) {
	log.Debugf("received get identity request")

	account, err := getAccount(ctx)
	if err != nil {
		return nil, err
	}
	data, err := account.Owner().Secret.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return &pb.GetIdentityResponse{
		Identity: data,
	}, nil
}

func (s *Service) CreateKey(ctx context.Context, req *pb.CreateKeyRequest) (*pb.CreateKeyResponse, error) {
	log.Debugf("received create key request")

	account, err := getAccount(ctx)
	if err != nil {
		return nil, err
	}
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
	key, err := s.Collections.APIKeys.Create(ctx, account.Owner().Key, keyType, req.Secure)
	if err != nil {
		return nil, err
	}
	t, err := keyTypeToPb(key.Type)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mapping key type: %v", key.Type)
	}

	var event analytics.Event
	if keyType == mdb.AccountKey {
		event = analytics.KeyAccountCreated
	} else {
		event = analytics.KeyUserCreated
	}
	if s.BillingClient != nil {
		payload := map[string]string{
			"secure_key": fmt.Sprintf("%t", req.Secure),
		}
		if account.User != nil {
			payload["member"] = account.User.Key.String()
			payload["member_username"] = account.User.Username
			payload["member_email"] = account.User.Email
		}
		// Same "member" based payload for Dev or Org account types so that same
		// downstream logic/templating can be used.
		s.BillingClient.TrackEvent(
			ctx,
			account.Owner().Key,
			account.Owner().Type,
			true,
			event,
			payload,
		)
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

	account, err := getAccount(ctx)
	if err != nil {
		return nil, err
	}
	key, err := s.Collections.APIKeys.Get(ctx, req.Key)
	if err != nil {
		return nil, err
	}
	if !account.Owner().Key.Equals(key.Owner) {
		return nil, status.Error(codes.PermissionDenied, "User does not own key")
	}
	if err := s.Collections.APIKeys.Invalidate(ctx, req.Key); err != nil {
		return nil, err
	}

	return &pb.InvalidateKeyResponse{}, nil
}

func (s *Service) ListKeys(ctx context.Context, _ *pb.ListKeysRequest) (*pb.ListKeysResponse, error) {
	log.Debugf("received list keys request")

	account, err := getAccount(ctx)
	if err != nil {
		return nil, err
	}
	keys, err := s.Collections.APIKeys.ListByOwner(ctx, account.Owner().Key)
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

	account, err := getAccount(ctx)
	if err != nil {
		return nil, err
	}
	if account.User == nil {
		return nil, status.Errorf(codes.InvalidArgument, errDevRequired.Error())
	}
	var powInfo *mdb.PowInfo
	if s.PowergateClient != nil {
		res, err := s.PowergateClient.Admin.Users.Create(s.powergateAdminCtx(ctx))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Unable to create user: %v", err)
		}
		powInfo = &mdb.PowInfo{ID: res.User.Id, Token: res.User.Token}
	}
	org, err := s.Collections.Accounts.CreateOrg(ctx, req.Name, []mdb.Member{{
		Key:      account.User.Key,
		Username: account.User.Username,
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

	if s.BillingClient != nil {
		// Identify the Org
		s.BillingClient.Identify(ctx, org.Key, org.Type, true, account.User.Email, map[string]string{
			"username":            org.Username,
			"created_by":          account.User.Key.String(),
			"created_by_username": account.User.Username,
			"created_by_email":    account.User.Email,
		})
		// Attribute event to Dev
		s.BillingClient.TrackEvent(
			ctx,
			account.User.Key,
			account.User.Type,
			true,
			analytics.OrgCreated,
			map[string]string{
				"org_name": org.Name,
				"org_key":  org.Key.String(),
			},
		)
	}

	return &pb.CreateOrgResponse{
		OrgInfo: orgInfo,
	}, nil
}

func (s *Service) GetOrg(ctx context.Context, _ *pb.GetOrgRequest) (*pb.GetOrgResponse, error) {
	log.Debugf("received get org request")

	account, err := getAccount(ctx)
	if err != nil {
		return nil, err
	}
	if account.Org == nil {
		return nil, status.Errorf(codes.FailedPrecondition, errOrgRequired.Error())
	}
	orgInfo, err := s.orgToPbOrg(account.Org)
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

	account, err := getAccount(ctx)
	if err != nil {
		return nil, err
	}
	if account.User == nil {
		return nil, status.Errorf(codes.InvalidArgument, errDevRequired.Error())
	}
	orgs, err := s.Collections.Accounts.ListByMember(ctx, account.User.Key)
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

	account, err := getAccount(ctx)
	if err != nil {
		return nil, err
	}
	if account.User == nil {
		return nil, status.Errorf(codes.InvalidArgument, errDevRequired.Error())
	}
	if account.Org == nil {
		return nil, status.Errorf(codes.InvalidArgument, errOrgRequired.Error())
	}
	isOwner, err := s.Collections.Accounts.IsOwner(ctx, account.Org.Username, account.User.Key)
	if err != nil {
		return nil, err
	}
	if !isOwner {
		return nil, status.Error(codes.PermissionDenied, "User must be an org owner")
	}

	if err = s.destroyAccount(ctx, account.Org); err != nil {
		return nil, err
	}

	if s.BillingClient != nil {
		s.BillingClient.TrackEvent(
			ctx,
			account.Owner().Key,
			account.Owner().Type,
			true,
			analytics.OrgDestroyed,
			map[string]string{
				"member":          account.User.Key.String(),
				"member_username": account.User.Username,
				"member_email":    account.User.Email,
			},
		)
	}

	return &pb.RemoveOrgResponse{}, nil
}

func (s *Service) InviteToOrg(ctx context.Context, req *pb.InviteToOrgRequest) (*pb.InviteToOrgResponse, error) {
	log.Debugf("received invite to org request")

	account, err := getAccount(ctx)
	if err != nil {
		return nil, err
	}
	if account.User == nil {
		return nil, status.Errorf(codes.InvalidArgument, errDevRequired.Error())
	}
	if account.Org == nil {
		return nil, status.Errorf(codes.InvalidArgument, errOrgRequired.Error())
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Email address in not valid")
	}
	invite, err := s.Collections.Invites.Create(ctx, account.User.Key, account.Org.Username, req.Email)
	if err != nil {
		return nil, err
	}

	ectx, cancel := context.WithTimeout(ctx, emailTimeout)
	defer cancel()
	if err = s.EmailClient.InviteAddress(
		ectx,
		account.User.Key.String(),
		account.Org.Name,
		account.User.Email,
		req.Email,
		s.GatewayURL,
		invite.Token,
	); err != nil {
		return nil, err
	}

	if s.BillingClient != nil {

		payload := map[string]string{
			"invitee": req.Email,
		}
		if account.User != nil {
			payload["member"] = account.User.Key.String()
			payload["member_username"] = account.User.Username
			payload["member_email"] = account.User.Email
		}
		s.BillingClient.TrackEvent(
			ctx,
			account.Owner().Key,
			account.Owner().Type,
			true,
			analytics.OrgInviteCreated,
			payload,
		)
	}

	return &pb.InviteToOrgResponse{Token: invite.Token}, nil
}

func (s *Service) LeaveOrg(ctx context.Context, _ *pb.LeaveOrgRequest) (*pb.LeaveOrgResponse, error) {
	log.Debugf("received leave org request")

	account, err := getAccount(ctx)
	if err != nil {
		return nil, err
	}
	if account.User == nil {
		return nil, status.Errorf(codes.InvalidArgument, errDevRequired.Error())
	}
	if account.Org == nil {
		return nil, status.Errorf(codes.InvalidArgument, errOrgRequired.Error())
	}
	if err := s.Collections.Accounts.RemoveMember(ctx, account.Org.Username, account.User.Key); err != nil {
		return nil, err
	}
	if err := s.Collections.Invites.DeleteByFromAndOrg(ctx, account.User.Key, account.Org.Username); err != nil {
		return nil, err
	}

	if s.BillingClient != nil {
		s.BillingClient.TrackEvent(
			ctx,
			account.Owner().Key,
			account.Owner().Type,
			true,
			analytics.OrgLeave,
			map[string]string{
				"member":          account.User.Key.String(),
				"member_username": account.User.Username,
				"member_email":    account.User.Email,
			},
		)
	}

	return &pb.LeaveOrgResponse{}, nil
}

func (s *Service) SetupBilling(ctx context.Context, _ *pb.SetupBillingRequest) (*pb.SetupBillingResponse, error) {
	log.Debugf("received setup billing request")

	if s.BillingClient == nil {
		return nil, fmt.Errorf("billing is not enabled")
	}
	account, err := getAccount(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.BillingClient.RecreateCustomerSubscription(
		ctx,
		account.Owner().Key,
	); err != nil {
		return nil, err
	}

	if s.BillingClient != nil {
		s.BillingClient.TrackEvent(
			ctx,
			account.Owner().Key,
			account.Owner().Type,
			true,
			analytics.BillingSetup,
			map[string]string{
				"member":          account.User.Key.String(),
				"member_username": account.User.Username,
				"member_email":    account.User.Email,
			},
		)
	}

	return &pb.SetupBillingResponse{}, nil
}

func (s *Service) GetBillingSession(
	ctx context.Context,
	_ *pb.GetBillingSessionRequest,
) (*pb.GetBillingSessionResponse, error) {
	log.Debugf("received get billing session request")

	if s.BillingClient == nil {
		return nil, fmt.Errorf("billing is not enabled")
	}
	account, err := getAccount(ctx)
	if err != nil {
		return nil, err
	}
	session, err := s.BillingClient.GetCustomerSession(ctx, account.Owner().Key)
	if err != nil {
		return nil, err
	}
	return &pb.GetBillingSessionResponse{Url: session.Url}, nil
}

func (s *Service) ListBillingUsers(
	ctx context.Context,
	req *pb.ListBillingUsersRequest,
) (*pb.ListBillingUsersResponse, error) {
	log.Debugf("received list billing users request")

	if s.BillingClient == nil {
		return nil, fmt.Errorf("billing is not enabled")
	}

	account, _ := mdb.AccountFromContext(ctx)
	list, err := s.BillingClient.ListDependentCustomers(
		ctx,
		account.Owner().Key,
		billing.WithOffset(req.Offset),
		billing.WithLimit(req.Limit),
	)
	if err != nil {
		return nil, err
	}

	return &pb.ListBillingUsersResponse{
		Users:      list.Customers,
		NextOffset: list.NextOffset,
	}, nil
}

func (s *Service) IsUsernameAvailable(
	ctx context.Context,
	req *pb.IsUsernameAvailableRequest,
) (*pb.IsUsernameAvailableResponse, error) {
	log.Debugf("received is username available request")

	if err := s.Collections.Accounts.IsUsernameAvailable(ctx, req.Username); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	return &pb.IsUsernameAvailableResponse{}, nil
}

func (s *Service) IsOrgNameAvailable(
	ctx context.Context,
	req *pb.IsOrgNameAvailableRequest,
) (*pb.IsOrgNameAvailableResponse, error) {
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

	account, err := getAccount(ctx)
	if err != nil {
		return nil, err
	}
	if account.User == nil {
		return nil, status.Errorf(codes.InvalidArgument, errDevRequired.Error())
	}
	if err := s.destroyAccount(ctx, account.User); err != nil {
		return nil, err
	}
	if s.BillingClient != nil {
		s.BillingClient.TrackEvent(
			ctx,
			account.Owner().Key,
			account.Owner().Type,
			true,
			analytics.AccountDestroyed,
			nil,
		)
	}

	return &pb.DestroyAccountResponse{}, nil
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
			bres, err := s.Threads.Find(
				ctx,
				t.ID,
				buckets.CollectionName,
				&db.Query{},
				&tdb.Bucket{},
				db.WithTxnToken(a.Token),
			)
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

func getAccount(ctx context.Context) (*mdb.AccountCtx, error) {
	account, _ := mdb.AccountFromContext(ctx)
	if account.Owner().Type == mdb.User {
		return nil, status.Errorf(codes.InvalidArgument, "account type not supported")
	}
	return account, nil
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
