package pkg

type ProtocolArg int

type ProtocolArgs []string

const (
	Target ProtocolArg = iota
	DNSResolver
)

func GetProtocolArgBase(target string) *ProtocolArgs {
	return &ProtocolArgs{target}
}

func GetProtocolArgDNS(target string, resolver string) *ProtocolArgs {
	return &ProtocolArgs{target, resolver}
}

func (p *ProtocolArgs) Target() string {
	return (*p)[Target]
}

func (p *ProtocolArgs) DNSResolver() string {
	return (*p)[DNSResolver]
}
