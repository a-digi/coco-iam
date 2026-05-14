package entity

type UserLogin interface {
	GetID() string
	GetUsername() string
	GetPassword() string
}

type LoginCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (c LoginCredentials) GetUsername() string { return c.Username }
func (c LoginCredentials) GetPassword() string { return c.Password }
