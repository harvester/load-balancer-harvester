package lbManager

type LB interface {
	EnsureEntry() (Entry, error)
	EnsureForwarder() (Forwarder, error)
}

type Entry interface {
	Ensure() error
}

type Forwarder interface {
	Ensure() error
}
