package app

import (
	"time"
)

type TUser struct {
	Name      string
	SId       int64
	LoginTime time.Time
}

func NewUser(userName string, userSID int64) *TUser {
	return &TUser{
		Name:      userName,
		SId:       userSID,
		LoginTime: time.Now(),
	}
}
