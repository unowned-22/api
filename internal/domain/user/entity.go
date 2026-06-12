package user

import "time"

type User struct {
	ID        int64
	Email     string
	Password  string
	RoleID    int64
	RoleName  string
	CreatedAt time.Time
}
