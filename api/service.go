package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/textileio/go-textile-core/broadcast"
	pb "github.com/textileio/textile/api/pb"
	"github.com/textileio/textile/messaging"
	"github.com/textileio/textile/resources/projects"
	"github.com/textileio/textile/resources/users"
)

var (
	loginTimeout = 3 * time.Minute
)

// service is a gRPC service for textile.
type service struct {
	users          *users.Users
	email          *messaging.EmailService
	bus            *broadcast.Broadcaster
	gatewayURL     string
	testUserSecret []byte
	projects       *projects.Projects
}

// Login handles a login request.
func (s *service) Login(req *pb.LoginRequest, stream pb.API_LoginServer) error {
	log.Debugf("received login request")
	matches, err := s.users.GetByEmail(req.Email)
	if err != nil {
		return err
	}

	var user = &users.User{}
	// @todo: can we ensure in threads that a model never >1 by field?
	if len(matches) == 0 {
		// create new user
		user = &users.User{Email: req.Email}
		if err := s.users.Create(user); err != nil {
			return err
		}
	} else {
		user = matches[0]
	}

	// create a single-use token
	var verification string
	if s.testUserSecret != nil {
		// enables token override for test-suite
		verification = string(s.testUserSecret)
	} else {
		verification, err = generateVerificationToken(48)
		if err != nil {
			return err
		}

	}

	// send challenge email
	err = s.email.VerifyAddress(user.Email, fmt.Sprintf("%s/verify/%s", s.gatewayURL, verification))
	if err != nil {
		return err
	}

	success := s.awaitVerification(verification)

	if success == false {
		return fmt.Errorf("email not verified")
	}

	token, err := generateAuthToken()
	if err != nil {
		return err
	}

	user.Token = token
	if err := s.users.Update(user); err != nil {
		return err
	}

	reply := &pb.LoginReply{
		ID:    user.ID,
		Token: token,
	}
	stream.Send(reply)
	return nil
}

func (s *service) awaitVerification(secret string) bool {
	listen := s.bus.Listen()
	ch := make(chan bool, 1)
	timer := time.NewTimer(loginTimeout)
	go func() {
		for i := range listen.Channel() {
			r, ok := i.(string)
			if ok {
				if r == secret {
					ch <- true
				}
			}
		}
	}()
	select {
	case ret := <-ch:
		listen.Discard()
		timer.Stop()
		return ret
	case <-timer.C:
		listen.Discard()
		return false
	}
}

func generateVerificationToken(size int) (string, error) {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
	rbytes := make([]byte, size)
	_, err := rand.Read(rbytes)
	if err != nil {
		return "", err
	}
	for i, b := range rbytes {
		rbytes[i] = letters[b%byte(len(letters))]
	}
	return base64.URLEncoding.EncodeToString(rbytes), err
}

func generateAuthToken() (string, error) {
	// @todo: finalize auth token design
	return generateVerificationToken(256)
}

// AddProject handles an add project request.
func (s *service) AddProject(ctx context.Context, req *pb.AddProjectRequest) (*pb.AddProjectReply, error) {
	log.Debugf("received add project request")

	// @todo: look up current user / team and get group ID
	// @todo: create store for the project

	project := &projects.Project{
		Name: req.Name,
	}
	if err := s.projects.Create(project); err != nil {
		return nil, err
	}

	return &pb.AddProjectReply{
		ID:      project.ID,
		StoreID: "", // @todo
	}, nil
}
