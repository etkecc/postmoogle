package email

// IncomingFilteringOptions for incoming mail
type IncomingFilteringOptions interface {
	SpamcheckDKIM() bool
	SpamcheckSMTP() bool
	SpamcheckSPF() bool
	SpamcheckMX() bool
	Spamlist() []string
}

// ContentOptions represents settings that specify how an email is to be converted to a Matrix message
type ContentOptions struct {
	// On/Off
	CC        bool
	Sender    bool
	Recipient bool
	Subject   bool
	HTML      bool
	Threads   bool
	Threadify bool

	// Keys
	MessageIDKey  string
	InReplyToKey  string
	ReferencesKey string
	SubjectKey    string
	FromKey       string
	ToKey         string
	CcKey         string
	RcptToKey     string
}
