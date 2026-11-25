package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

type mentionsJSONOutput struct {
	Entry    jsonNodeInfo      `json:"entry"`
	Incoming []jsonMention     `json:"incoming"`
	Outgoing *[]jsonMention    `json:"outgoing,omitempty"`
	Total    jsonMentionsTotal `json:"total"`
}

type jsonMention struct {
	jsonNodeInfo
	MatchedText string           `json:"matched_text"`
	Location    jsonLinkLocation `json:"location"`
	Excerpt     string           `json:"excerpt"`
}

type jsonMentionsTotal struct {
	Incoming int  `json:"incoming"`
	Outgoing *int `json:"outgoing,omitempty"`
}

func toJSONMentions(mentions []core.Mention) []jsonMention {
	items := make([]jsonMention, len(mentions))
	for i, m := range mentions {
		items[i] = jsonMention{
			jsonNodeInfo: toJSONNodeInfo(m.NodeInfo),
			MatchedText:  m.MatchedText,
			Location: jsonLinkLocation{
				Start: jsonLinkPosition{m.Location.Start.Line, m.Location.Start.Column},
				End:   jsonLinkPosition{m.Location.End.Line, m.Location.End.Column},
			},
			Excerpt: m.Excerpt,
		}
	}
	return items
}

func printMentionsJSON(w io.Writer, r *core.MentionsResult) error {
	out := mentionsJSONOutput{
		Entry:    toJSONNodeInfo(r.Entry),
		Incoming: toJSONMentions(r.Incoming),
		Total:    jsonMentionsTotal{Incoming: r.Total.Incoming, Outgoing: r.Total.Outgoing},
	}
	if r.Outgoing != nil {
		items := toJSONMentions(r.Outgoing)
		out.Outgoing = &items
	}
	return encodeJSON(w, out)
}

func printMentionsText(w io.Writer, r *core.MentionsResult) error {
	output := &textErrorWriter{w: w}
	w = output
	fmt.Fprintln(w, "entry:")
	writeQueryNodeText(w, r.Entry, "  ")
	writeMentionsText(w, "incoming", r.Incoming)
	if r.Outgoing != nil {
		writeMentionsText(w, "outgoing", r.Outgoing)
	}
	fmt.Fprintln(w, "total:")
	fmt.Fprintf(w, "  incoming: %d\n", r.Total.Incoming)
	if r.Total.Outgoing != nil {
		fmt.Fprintf(w, "  outgoing: %d\n", *r.Total.Outgoing)
	}
	return output.err
}

func writeMentionsText(w io.Writer, label string, mentions []core.Mention) {
	fmt.Fprintf(w, "%s:\n", label)
	if len(mentions) == 0 {
		fmt.Fprintln(w, "  （該当なし）")
	}
	for _, m := range mentions {
		writeQueryNodeText(w, m.NodeInfo, "  ")
		matched, _ := json.Marshal(m.MatchedText)
		excerpt, _ := json.Marshal(m.Excerpt)
		loc := m.Location
		fmt.Fprintf(w, "    matched_text: %s\n    location: %d:%d-%d:%d\n    excerpt: %s\n", matched, loc.Start.Line, loc.Start.Column, loc.End.Line, loc.End.Column, excerpt)
	}
}
