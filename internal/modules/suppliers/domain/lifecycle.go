package domain

// Lifecycle transitions are idempotent and never affect related entities.
func (s *Supplier) Activate() bool   { return setActive(&s.Active, true) }
func (s *Supplier) Deactivate() bool { return setActive(&s.Active, false) }
func (b *Branch) Activate() bool     { return setActive(&b.Active, true) }
func (b *Branch) Deactivate() bool   { return setActive(&b.Active, false) }
func (c *Contact) Activate() bool    { return setActive(&c.Active, true) }
func (c *Contact) Deactivate() bool  { return setActive(&c.Active, false) }

func setActive(current *bool, target bool) bool {
	if *current == target {
		return false
	}
	*current = target
	return true
}
