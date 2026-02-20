package whatsapp

import "time"

// Account represents WhatsApp Business Account credentials
type Account struct {
	PhoneID     string
	BusinessID  string
	AppID       string
	APIVersion  string
	AccessToken string
}

// Button represents an interactive button
type Button struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type,omitempty"` // "reply" (default) or "url"
	URL   string `json:"url,omitempty"`  // URL for type="url" buttons
}

// MetaAPIResponse represents a successful API response from Meta
type MetaAPIResponse struct {
	Messages []struct {
		ID string `json:"id"`
	} `json:"messages"`
}

// MetaAPIError represents an error response from Meta API
type MetaAPIError struct {
	Error struct {
		Message      string `json:"message"`
		Type         string `json:"type"`
		Code         int    `json:"code"`
		ErrorSubcode int    `json:"error_subcode"`
		ErrorUserMsg string `json:"error_user_msg"`
		ErrorData    struct {
			Details string `json:"details"`
		} `json:"error_data"`
		FBTraceID string `json:"fbtrace_id"`
	} `json:"error"`
}

// TemplateResponse represents response from template submission
type TemplateResponse struct {
	ID string `json:"id"`
}

// MetaTemplate represents a template fetched from Meta
type MetaTemplate struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Language   string              `json:"language"`
	Category   string              `json:"category"`
	Status     string              `json:"status"`
	Components []TemplateComponent `json:"components"`
}

// TemplateComponent represents a component of a template
type TemplateComponent struct {
	Type    string           `json:"type"`
	Format  string           `json:"format,omitempty"`
	Text    string           `json:"text,omitempty"`
	Buttons []TemplateButton `json:"buttons,omitempty"`
	Example *TemplateExample `json:"example,omitempty"`
}

// TemplateButton represents a button in a template
type TemplateButton struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	URL         string `json:"url,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
	Example     any    `json:"example,omitempty"`
}

// TemplateExample represents example values for template variables
type TemplateExample struct {
	HeaderText   []string   `json:"header_text,omitempty"`
	HeaderHandle []string   `json:"header_handle,omitempty"`
	BodyText     [][]string `json:"body_text,omitempty"`
}

// TemplateListResponse represents response from fetching templates
type TemplateListResponse struct {
	Data []MetaTemplate `json:"data"`
}

// WebhookPayload represents the incoming webhook from Meta
type WebhookPayload struct {
	Object string         `json:"object"`
	Entry  []WebhookEntry `json:"entry"`
}

// WebhookEntry represents an entry in the webhook payload
type WebhookEntry struct {
	ID      string          `json:"id"`
	Changes []WebhookChange `json:"changes"`
}

// WebhookChange represents a change in the webhook entry
type WebhookChange struct {
	Value WebhookValue `json:"value"`
	Field string       `json:"field"`
}

// WebhookValue represents the value of a webhook change
type WebhookValue struct {
	MessagingProduct string           `json:"messaging_product"`
	Metadata         WebhookMetadata  `json:"metadata"`
	Contacts         []WebhookContact `json:"contacts,omitempty"`
	Messages         []WebhookMessage `json:"messages,omitempty"`
	Statuses         []WebhookStatus  `json:"statuses,omitempty"`
}

// WebhookMetadata represents metadata in webhook
type WebhookMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

// WebhookContact represents a contact in webhook
type WebhookContact struct {
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
	WaID string `json:"wa_id"`
}

// WebhookMessage represents an incoming message
type WebhookMessage struct {
	From        string                  `json:"from"`
	ID          string                  `json:"id"`
	Timestamp   string                  `json:"timestamp"`
	Type        string                  `json:"type"`
	Text        *WebhookText            `json:"text,omitempty"`
	Interactive *WebhookInteractive     `json:"interactive,omitempty"`
	Image       *WebhookMedia           `json:"image,omitempty"`
	Document    *WebhookMedia           `json:"document,omitempty"`
	Audio       *WebhookMedia           `json:"audio,omitempty"`
	Video       *WebhookMedia           `json:"video,omitempty"`
	Context     *WebhookMessageContext  `json:"context,omitempty"`
}

// WebhookText represents text content in a message
type WebhookText struct {
	Body string `json:"body"`
}

// WebhookInteractive represents interactive message response
type WebhookInteractive struct {
	Type        string              `json:"type"`
	ButtonReply *WebhookButtonReply `json:"button_reply,omitempty"`
	ListReply   *WebhookListReply   `json:"list_reply,omitempty"`
	NFMReply    *WebhookNFMReply    `json:"nfm_reply,omitempty"`
}

// WebhookButtonReply represents a button reply
type WebhookButtonReply struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// WebhookListReply represents a list selection reply
type WebhookListReply struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// WebhookNFMReply represents a flow reply
type WebhookNFMReply struct {
	ResponseJSON string `json:"response_json"`
	Body         string `json:"body"`
	Name         string `json:"name"`
}

// WebhookMedia represents media in a message
type WebhookMedia struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	Caption  string `json:"caption,omitempty"`
	Filename string `json:"filename,omitempty"`
}

// WebhookMessageContext represents message context (for replies)
type WebhookMessageContext struct {
	From      string `json:"from"`
	ID        string `json:"id"`
	Forwarded bool   `json:"forwarded,omitempty"`
}

// WebhookStatus represents a message status update
type WebhookStatus struct {
	ID          string               `json:"id"`
	Status      string               `json:"status"`
	Timestamp   string               `json:"timestamp"`
	RecipientID string               `json:"recipient_id"`
	Errors      []WebhookStatusError `json:"errors,omitempty"`
}

// WebhookStatusError represents an error in status update
type WebhookStatusError struct {
	Code    int    `json:"code"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

// ParsedMessage represents a parsed incoming message
type ParsedMessage struct {
	From          string
	ID            string
	Timestamp     time.Time
	Type          string
	Text          string
	ButtonReplyID string
	ListReplyID   string
	MediaID       string
	MediaMimeType string
	Caption       string
	ContactName   string
	PhoneNumberID string
}

// ParsedStatus represents a parsed status update
type ParsedStatus struct {
	MessageID   string
	Status      string
	Timestamp   time.Time
	RecipientID string
	ErrorCode   int
	ErrorTitle  string
	ErrorMsg    string
}

// CatalogInfo represents a catalog from Meta API
type CatalogInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CatalogListResponse represents response from listing catalogs
type CatalogListResponse struct {
	Data []CatalogInfo `json:"data"`
}

// ProductInput represents input for creating/updating a product
type ProductInput struct {
	Name        string `json:"name"`
	Price       int64  `json:"price"`    // Price in cents
	Currency    string `json:"currency"`
	URL         string `json:"url"`
	ImageURL    string `json:"image_url"`
	RetailerID  string `json:"retailer_id"` // SKU
	Description string `json:"description"`
}

// ProductInfo represents a product from Meta API
type ProductInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Price       string `json:"price"`
	Currency    string `json:"currency"`
	URL         string `json:"url"`
	ImageURL    string `json:"image_url"`
	RetailerID  string `json:"retailer_id"`
	Description string `json:"description"`
}

// ProductListResponse represents response from listing products
type ProductListResponse struct {
	Data []ProductInfo `json:"data"`
}

// ProductCreateResponse represents response from creating a product
type ProductCreateResponse struct {
	ID string `json:"id"`
}

// BusinessProfile represents the business profile of a phone number
type BusinessProfile struct {
	MessagingProduct string   `json:"messaging_product"`
	Address          string   `json:"address"`
	Description      string   `json:"description"`
	Vertical         string   `json:"vertical"`
	Email            string   `json:"email"`
	Websites         []string `json:"websites"`
	ProfilePicture   string   `json:"profile_picture_url"`
	About            string   `json:"about"` // Status text
}

// BusinessProfileInput represents the input for updating a business profile
type BusinessProfileInput struct {
	MessagingProduct     string   `json:"messaging_product"`
	Address              string   `json:"address,omitempty"`
	Description          string   `json:"description,omitempty"`
	Vertical             string   `json:"vertical,omitempty"`
	Email                string   `json:"email,omitempty"`
	Websites             []string `json:"websites,omitempty"`
	ProfilePictureHandle string   `json:"profile_picture_handle,omitempty"`
	About                string   `json:"about,omitempty"`
}
