package identity

type Admin interface {
	UID() string
	Email() string
	PasswordHash() string
}

type admin struct {
	uid          string
	email        string
	passwordHash string
}

func (a *admin) UID() string          { return a.uid }
func (a *admin) Email() string        { return a.email }
func (a *admin) PasswordHash() string { return a.passwordHash }

// Restore reconstrói um admin já persistido.
func Restore(uid, email, passwordHash string) Admin {
	return &admin{uid: uid, email: email, passwordHash: passwordHash}
}
