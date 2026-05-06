package main

import (
	"fmt"

	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/zerodha/fastglue"
)

const (
	minSearchQueryLength = 3
)

// handleSearchConversations searches conversations based on the query.
func handleSearchConversations(r *fastglue.Request) error {
	app := r.Context.(*App)
	wrapper := func(query string) (interface{}, error) {
		return app.search.Conversations(query)
	}
	return handleSearch(r, wrapper)
}

// handleSearchMessages searches messages based on the query.
func handleSearchMessages(r *fastglue.Request) error {
	app := r.Context.(*App)
	wrapper := func(query string) (interface{}, error) {
		return app.search.Messages(query)
	}
	return handleSearch(r, wrapper)
}

// handleSearchContacts searches contacts based on the query.
func handleSearchContacts(r *fastglue.Request) error {
	app := r.Context.(*App)
	wrapper := func(query string) (interface{}, error) {
		return app.search.Contacts(query)
	}
	return handleSearch(r, wrapper)
}

// handleUnifiedSearch performs a single search across conversations, subject,
// reference number, contact email, and message text content. Returns paginated
// results — see FS8.
func handleUnifiedSearch(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		q   = string(r.RequestCtx.QueryArgs().Peek("query"))
	)

	if len(q) < minSearchQueryLength {
		return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.Ts("search.minQueryLength", "length", fmt.Sprintf("%d", minSearchQueryLength)), nil))
	}

	page, pageSize := getSearchPagination(r)
	results, err := app.search.Unified(q, page, pageSize)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(results)
}

// getSearchPagination returns 1-indexed page and page_size with sane defaults
// (page=1, page_size=30, capped at 100). Kept narrow to the search package
// to avoid colliding with any future generic pagination helper.
func getSearchPagination(r *fastglue.Request) (page, pageSize int) {
	page = r.RequestCtx.QueryArgs().GetUintOrZero("page")
	if page < 1 {
		page = 1
	}
	pageSize = r.RequestCtx.QueryArgs().GetUintOrZero("page_size")
	if pageSize < 1 {
		pageSize = 30
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// handleSearch searches for the given query using the provided search function.
func handleSearch(r *fastglue.Request, searchFunc func(string) (interface{}, error)) error {
	var (
		app = r.Context.(*App)
		q   = string(r.RequestCtx.QueryArgs().Peek("query"))
	)

	if len(q) < minSearchQueryLength {
		return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.Ts("search.minQueryLength", "length", fmt.Sprintf("%d", minSearchQueryLength)), nil))
	}

	results, err := searchFunc(q)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(results)
}
