package project

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GitHub keeps a CHANGES_REQUESTED verdict until the reviewer approves or the
// review is dismissed, so the verdict alone cannot tell a pull request waiting
// on its author from one waiting on its reviewer: the threads can. gh's
// `pr list --json` has no field for them, so one GraphQL call asks for every
// row with changes requested at once, each under its own alias, and a list
// with none costs no call at all.
//
// Ceiling: the last 100 threads of each pull request, the conversation's own
// cap (prthreads.go).

// ThreadTally is how many of a pull request's review threads are still open.
type ThreadTally struct {
	Open  int `json:"open"`
	Total int `json:"total"`
}

// ghThreadTallies mirrors the aliased payload: one key per pull request asked.
type ghThreadTallies struct {
	Data struct {
		Repository map[string]*struct {
			Number        int `json:"number"`
			ReviewThreads struct {
				Nodes []struct {
					IsResolved bool `json:"isResolved"`
				} `json:"nodes"`
			} `json:"reviewThreads"`
		} `json:"repository"`
	} `json:"data"`
}

// tallyRequestedThreads fills Threads on the rows whose verdict is
// CHANGES_REQUESTED; the other rows keep a zero tally.
func (s *Service) tallyRequestedThreads(path string, list []PRSummary) error {
	var numbers []int
	for _, pr := range list {
		if pr.ReviewDecision == "CHANGES_REQUESTED" {
			numbers = append(numbers, pr.Number)
		}
	}
	if len(numbers) == 0 {
		return nil
	}
	out, err := s.gh(prReadTimeout, path, "api", "graphql",
		"-F", "owner={owner}", "-F", "repo={repo}",
		"-f", "query="+threadTallyQuery(numbers))
	if err != nil {
		return err
	}
	tallies, err := parseThreadTallies(out)
	if err != nil {
		return err
	}
	for i := range list {
		if t, ok := tallies[list[i].Number]; ok {
			list[i].Threads = t
		}
	}
	return nil
}

// threadTallyQuery asks for each number's threads under the alias pr<N>; the
// numbers are ints from gh's own answer, so nothing typed reaches the query.
func threadTallyQuery(numbers []int) string {
	var b strings.Builder
	b.WriteString(`query($owner:String!,$repo:String!){repository(owner:$owner,name:$repo){`)
	for _, n := range numbers {
		fmt.Fprintf(&b, "pr%d:pullRequest(number:%d){number reviewThreads(last:100){nodes{isResolved}}}", n, n)
	}
	b.WriteString("}}")
	return b.String()
}

func parseThreadTallies(out []byte) (map[int]ThreadTally, error) {
	var payload ghThreadTallies
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, errUnreadableAnswer(err)
	}
	tallies := make(map[int]ThreadTally, len(payload.Data.Repository))
	for _, pr := range payload.Data.Repository {
		if pr == nil {
			continue
		}
		t := ThreadTally{Total: len(pr.ReviewThreads.Nodes)}
		for _, n := range pr.ReviewThreads.Nodes {
			if !n.IsResolved {
				t.Open++
			}
		}
		tallies[pr.Number] = t
	}
	return tallies, nil
}
