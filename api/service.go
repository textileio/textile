package api

import (
	"fmt"
	"time"

	"github.com/textileio/go-textile-core/broadcast"
	pb "github.com/textileio/textile/api/pb"
	"github.com/textileio/textile/messaging"
	"github.com/textileio/textile/resources/users"
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
			// TODO: timeout close
		}()
		// wrap with a simple 1s timeout
		select {
		case ret := <-sub:
			ch <- ret
		case <-time.After(30 * time.Second):
			ch <- false
		}
	}()
	return ch
}

// Login handles a login request.
func (s *service) Login(req *pb.LoginRequest, stream pb.API_LoginServer) error {
	log.Debugf("received login request")
	matches, err := s.users.GetByEmail(req.Email)
	if err != nil {
		return err
	}

	var user = &users.User{}
	// @todo: how to ensure never >1
	if len(matches) == 0 {
		// create new user
		user = &users.User{Email: req.Email}
		if err := s.users.Create(user); err != nil {
			return err
		}
	} else {
		user = matches[0]
	}

	// create 1-time-use secret
	secret := "acz01r4i89skel3kls"

	// send challenge email
	err = s.email.VerifyAddress(user.Email, fmt.Sprintf("%s/verify/%s", s.gatewayURL, secret))
	if err != nil {
		return err
	}

	ch := s.awaitVerification(secret)
	success := <-ch

	if success == false {
		return fmt.Errorf("email not verified")
	}

	token := "buffalo sauce"

	// @todo: this should be an array (?)
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
