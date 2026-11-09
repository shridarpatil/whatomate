package mcp

func init() {
	registerArea(Area{
		Key:         "flow_media",
		Title:       "Chatbot flow media",
		Description: "Upload images, videos and documents used inside chatbot flow steps.",
	})

	register(Entry{
		Area:   "flow_media",
		Method: "POST",
		Path:   "/api/flow-media/upload",
		Summary: "Upload a media file (max 16 MB) for a chatbot flow step. Returns a public URL to " +
			"put in the step's media settings.",
		Multipart: &MultipartSpec{
			FileField: "file",
			Accept:    "image/*,video/*,audio/*,application/pdf",
		},
	})
}
