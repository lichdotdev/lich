package project

// PRReviewer is one entry of a pull request's review roster: somebody who was
// asked and has not answered yet, or somebody who has.
type PRReviewer struct {
	// Login is the account's login, or a team's slug — which is not what gh takes
	// to add or remove one (that is org/slug), so a team is shown and not edited.
	Login  string `json:"login"`
	IsTeam bool   `json:"isTeam"`
	// State is "" while the review is still owed, else gh's APPROVED |
	// CHANGES_REQUESTED | COMMENTED | DISMISSED.
	State string `json:"state"`
}

// ghReviewRequest is one entry of gh's reviewRequests: a User carries a login, a
// Team a slug, and __typename is the only thing that says which.
type ghReviewRequest struct {
	TypeName string `json:"__typename"`
	Login    string `json:"login"`
	Slug     string `json:"slug"`
}

// ghLatestReview is one entry of gh's latestReviews — the last review each
// person left.
type ghLatestReview struct {
	Author ghPRAuthor `json:"author"`
	State  string     `json:"state"`
}

// toReviewers merges gh's two halves of the roster into one list. They have to
// be read together: a submitted review clears its request, so the pending half
// alone reads as "nobody is reviewing this" the moment the first review lands,
// and the answered half alone hides everyone still owing one.
func toReviewers(reviews []ghLatestReview, requests []ghReviewRequest) []PRReviewer {
	out := make([]PRReviewer, 0, len(reviews)+len(requests))
	at := make(map[string]int, len(reviews))
	for _, r := range reviews {
		login := firstNonEmpty(r.Author.Login, r.Author.Name)
		if login == "" {
			continue
		}
		at[login] = len(out)
		out = append(out, PRReviewer{Login: login, State: r.State})
	}
	for _, req := range requests {
		login := firstNonEmpty(req.Login, req.Slug)
		if login == "" {
			continue
		}
		// Asked again after reviewing: GitHub keeps the old verdict in
		// latestReviews and re-files the request, and what stands now is the
		// answer owed, not the one already given.
		if i, ok := at[login]; ok {
			out[i].State = ""
			continue
		}
		out = append(out, PRReviewer{Login: login, IsTeam: req.TypeName == "Team"})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
