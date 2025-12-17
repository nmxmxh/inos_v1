package core

// Credits is a simple economy type for tracking earned/spent credits.
type Credits struct {
	Balance int64
}

// Add increases the credit balance.
func (c *Credits) Add(amount int64) {
	c.Balance += amount
}

// Spend decreases the credit balance.
func (c *Credits) Spend(amount int64) bool {
	if c.Balance < amount {
		return false
	}
	c.Balance -= amount
	return true
}
