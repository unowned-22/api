package bootstrap

import (
	"embed"
	"fmt"
	"time"

	"github.com/unowned-22/api/internal/domain/user"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

//go:embed fixtures/*.yaml
var fixturesFS embed.FS

type yamlUser struct {
	Email         string `yaml:"email"`
	Username      string `yaml:"username"`
	FullName      string `yaml:"full_name"`
	Phone         string `yaml:"phone"`
	PlainPassword string `yaml:"plain_password"`
}

type yamlFriendship struct {
	RequesterUsername string `yaml:"requester_username"`
	AddresseeUsername string `yaml:"addressee_username"`
	Status            string `yaml:"status"`
}

type yamlUserRoot struct {
	Users       []yamlUser       `yaml:"users"`
	Friendships []yamlFriendship `yaml:"friendships"`
}

type Fixtures struct {
	Users       []*user.User
	Friendships []yamlFriendship
}

func LoadFixtures() (*Fixtures, error) {
	data, err := fixturesFS.ReadFile("fixtures/data.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read users fixture file: %w", err)
	}

	var root yamlUserRoot
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("failed to unmarshal users yaml: %w", err)
	}

	var outUsers []*user.User
	for _, yu := range root.Users {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(yu.PlainPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password for %s: %w", yu.Email, err)
		}

		u := &user.User{
			Email:     yu.Email,
			Username:  yu.Username,
			FullName:  yu.FullName,
			Phone:     yu.Phone,
			Password:  string(hashedPassword),
			CreatedAt: time.Now(),
		}
		outUsers = append(outUsers, u)
	}

	return &Fixtures{
		Users:       outUsers,
		Friendships: root.Friendships,
	}, nil
}
