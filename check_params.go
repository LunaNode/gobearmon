package gobearmon

type HttpCheckParams struct {
	Url string `json:"url"`
	Method string `json:"method"`
	Body string `json:"body"`
	Headers map[string]string `json:"headers"`
	Timeout int `json:"timeout"`
	Insecure bool `json:"insecure"`
	Username string `json:"username"`
	Password string `json:"password"`

	ExpectStatus int `json:"expect_status"`
	ExpectSubstring string `json:"expect_substring"`
}

type TcpCheckParams struct {
	Address string `json:"address"`
	Timeout int `json:"timeout"`
	Payload string `json:"payload"`
	ForceIP int `json:"force_ip"`

	Expect string `json:"expect"`
}

type IcmpCheckParams struct {
	Target string `json:"target"`
	PacketLoss bool `json:"packetloss"`
	ForceIP int `json:"force_ip"`
}

type SslExpireCheckParams struct {
	Address string `json:"address"`
	Days int `json:"days"`
}
