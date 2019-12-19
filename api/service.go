package api

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/textileio/go-textile-core/broadcast"
	pb "github.com/textileio/textile/api/pb"
	"github.com/textileio/textile/messaging"
	"github.com/textileio/textile/resources/users"
)

var (
	loginTimeout = 120 * time.Second
)

// service is a gRPC service for textile.
type service struct {
	users      *users.Users
	email      *messaging.EmailService
	bus        *broadcast.Broadcaster
	gatewayURL string
	//projects *projects.Projects
}

func (s *service) awaitVerification(secret string) chan bool {
	// @todo match supplied secret, not just true/false
	listen := s.bus.Listen()
	ch := make(chan bool, 1)
	go func() {
		sub := make(chan bool, 1)
		go func() {
			for i := range listen.Channel() {
				r, ok := i.(string)
				if ok {
					if r == secret {
						fmt.Println("true")
						sub <- true
					}
				}
			}
		}()
		select {
		case ret := <-sub:
			ch <- ret
		case <-time.After(loginTimeout):
			ch <- false
		}
	}()
	return ch
}

func (s *service) generateVerificationToken(size int) (string, error) {
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

func (s *service) generateAuthToken() (string, error) {
	// @todo: finalize auth token design
	return s.generateVerificationToken(256)
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
	verification, err := s.generateVerificationToken(48)
	if err != nil {
		return err
	}

	// send challenge email
	err = s.email.VerifyAddress(user.Email, fmt.Sprintf("%s/verify/%s", s.gatewayURL, verification))
	if err != nil {
		return err
	}

	ch := s.awaitVerification(string(verification))
	success := <-ch

	if success == false {
		return fmt.Errorf("email not verified")
	}

	token, err := s.generateAuthToken()
	if err != nil {
		return err
	}

	user.Token = token
	if err := s.users.Save(user); err != nil {
		return err
	}

	reply := &pb.LoginReply{
		ID:    user.ID,
		Token: token,
	}
	stream.Send(reply)
	return nil
}
