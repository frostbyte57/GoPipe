package mailbox

// Client-to-Server messages
type genericOutMessage struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"` // Request ID for correlation
}

type BindMessage struct {
	Type  string `json:"type"`
	ID    string `json:"id,omitempty"`
	AppID string `json:"appid"`
	Side  string `json:"side"`
}

type ListMessage struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
}

type AllocateMessage struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
}

type ClaimMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id,omitempty"`
	Nameplate string `json:"nameplate"`
}

type ReleaseMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id,omitempty"`
	Nameplate string `json:"nameplate,omitempty"`
}

type OpenMessage struct {
	Type    string `json:"type"`
	ID      string `json:"id,omitempty"`
	Mailbox string `json:"mailbox"`
}

type CloseMessage struct {
	Type    string `json:"type"`
	ID      string `json:"id,omitempty"`
	Mailbox string `json:"mailbox,omitempty"`
	Mood    string `json:"mood,omitempty"`
}

type AddMessage struct {
	Type  string `json:"type"`
	ID    string `json:"id,omitempty"`
	Phase string `json:"phase"`
	Body  string `json:"body"` // Hex encoded body usually
}

type PingMessage struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
	Ping int    `json:"ping"`
}

// Server-to-Client messages
type GenericInMessage struct {
	Type string `json:"type"`
}

type WelcomeMessage struct {
	Type    string `json:"type"`
	Welcome struct {
		MOTD              string `json:"motd"`
		CurrentCLIUtility string `json:"current_cli_utility"`
	} `json:"welcome"`
}

type AllocatedMessage struct {
	Type      string `json:"type"`
	Nameplate string `json:"nameplate"`
}

type ClaimedMessage struct {
	Type    string `json:"type"`
	Mailbox string `json:"mailbox"`
}

type MessageMessage struct {
	Type  string `json:"type"`
	Side  string `json:"side"`
	Phase string `json:"phase"`
	ID    string `json:"id"`
	Body  string `json:"body"`
}
