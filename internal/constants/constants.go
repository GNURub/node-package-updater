package constants

type DepEnv string

const (
	PackageManager   DepEnv = "packageManager"
	Dependencies     DepEnv = "dependencies"
	DevDependencies  DepEnv = "devDependencies"
	PeerDependencies DepEnv = "peerDependencies"
)

func (t DepEnv) String() string {
	return string(t)
}

func (t DepEnv) ToEnv() string {
	switch t {
	case PackageManager:
		return "packageManager"
	case Dependencies:
		return "production"
	case DevDependencies:
		return "development"
	case PeerDependencies:
		return "peer"
	default:
		return ""
	}
}
