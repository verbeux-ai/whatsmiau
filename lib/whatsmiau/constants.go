package whatsmiau

type Status string

const (
	Connected  = "open"
	Connecting = "connecting"
	QrCode     = "qr-code"
	Closed     = "closed"
)
