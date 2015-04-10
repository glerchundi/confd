package backends

type Config struct {
	Backend       string
	ClientCaKeys  string
	ClientCert    string
	ClientKey     string
	BackendNodes  []string
	Scheme        string
	FsRootPath    string
	FsMaxFileSize int
}
