package main

import "github.com/lichdotdev/lich/api"

// notesRequest is the notes as a request: each element's label, page and
// selector, the React component that drew it, its markup and the person's
// note, the sketch at image when there is one, drawn on the page at url, and
// closing, the note about all of them.
func notesRequest(session string, notes []*browserNote, image, url, closing string) api.RequestOptions {
	req := api.RequestOptions{SessionID: session, Topic: api.RequestBrowserNotes, Image: image, URL: url, Note: closing}
	for _, n := range notes {
		el := n.el
		req.Items = append(req.Items, api.RequestItem{Label: el.Label, URL: el.URL, Selector: el.Selector,
			Component: el.Component, Source: el.Source, HTML: el.HTML, Text: n.text})
	}
	if req.URL == "" && len(notes) > 0 {
		req.URL = notes[0].el.URL
	}
	return req
}
