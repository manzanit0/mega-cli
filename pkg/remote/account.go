package remote

// Account describes the logged-in user.
type Account struct {
	// Email is the account email.
	Email string `json:"email"`
	// Name is the account display name.
	Name string `json:"name"`
	// Handle is the MEGA user handle.
	Handle string `json:"handle"`
}

// Storage reports account capacity.
type Storage struct {
	// Used is the number of bytes stored.
	Used uint64 `json:"used"`
	// Total is the account capacity in bytes.
	Total uint64 `json:"total"`
	// Free is the remaining capacity in bytes.
	Free uint64 `json:"free"`
}

// Percent returns the share of capacity in use, from 0 to 100.
func (s Storage) Percent() float64 {
	if s.Total == 0 {
		return 0
	}
	return float64(s.Used) * 100 / float64(s.Total)
}

// Account returns the logged-in user's details.
func (c *Client) Account() (Account, error) {
	u, err := c.User()
	if err != nil {
		return Account{}, err
	}
	return Account{Email: u.Email, Name: u.Name, Handle: u.U}, nil
}

// Storage returns the account's storage usage.
func (c *Client) Storage() (Storage, error) {
	q, err := c.Quota()
	if err != nil {
		return Storage{}, err
	}
	s := Storage{Used: q.Cstrg, Total: q.Mstrg}
	if s.Total > s.Used {
		s.Free = s.Total - s.Used
	}
	return s, nil
}
