package config

// Options for the admitter webhook server
type Options struct {
	Namespace       string
	Threadiness     int
	HTTPSListenPort int

	ControllerUsername        string
	GarbageCollectionUsername string
}
