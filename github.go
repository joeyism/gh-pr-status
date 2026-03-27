package main

import (
	"context"
	"errors"
	"time"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

func getGitHubToken() (string, error) {
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		return tok, nil
	}
	if tok := os.Getenv("GH_TOKEN"); tok != "" {
		return tok, nil
	}

	path, err := exec.LookPath("gh")
	if err != nil {
		return "", errors.New("gh CLI not found; set GITHUB_TOKEN or install gh")
	}

	cmd := exec.Command(path, "auth", "token")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run gh auth token: %w", err)
	}

	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", errors.New("gh auth token returned empty; run 'gh auth login' first")
	}

	return token, nil
}

func newGitHubClient(token string) *githubv4.Client {
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(context.Background(), src)
	return githubv4.NewClient(httpClient)
}

type CheckRun struct {
	Name       string
	Status     string
	Conclusion string
}

type PullRequest struct {
	ID                string
	Number            int
	Title             string
	URL               string
	Repo              string
	Org               string
	Author            string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	CheckStatus       string
	CheckRuns         []CheckRun
	ReviewDecision    string
	IsDraft           bool
	TotalComments     int
	UnresolvedThreads int
	TotalThreads      int
	Mergeable         string
}

// countUnresolved counts unresolved review threads from a node slice.
// Returns -1 if totalCount > len(nodes), indicating the count is truncated.
func countUnresolved(nodes []struct{ IsResolved githubv4.Boolean }, totalCount int) int {
	count := 0
	for _, t := range nodes {
		if !bool(t.IsResolved) {
			count++
		}
	}
	if totalCount > len(nodes) {
		return -1
	}
	return count
}

func getViewerLogin(ctx context.Context, client *githubv4.Client) (string, error) {
	var q struct {
		Viewer struct {
			Login githubv4.String
		}
	}

	if err := client.Query(ctx, &q, nil); err != nil {
		return "", err
	}
	return string(q.Viewer.Login), nil
}

func fetchPRs(ctx context.Context, client *githubv4.Client, username string, orgs []string) ([]PullRequest, error) {
	var sb strings.Builder
	sb.WriteString("is:pr is:open archived:false author:")
	sb.WriteString(username)
	for _, org := range orgs {
		if org == "" {
			continue
		}
		sb.WriteString(" org:")
		sb.WriteString(org)
	}

	var q struct {
		Search struct {
			Nodes []struct {
				PullRequest struct {
					ID                 githubv4.ID
					Number             githubv4.Int
					Title              githubv4.String
					URL                githubv4.String
					IsDraft            githubv4.Boolean
					ReviewDecision     githubv4.String
					Mergeable          githubv4.String
					TotalCommentsCount githubv4.Int
					Author             struct {
						Login githubv4.String
					}
					CreatedAt githubv4.DateTime
					UpdatedAt githubv4.DateTime
					ReviewThreads struct {
						TotalCount githubv4.Int
						Nodes      []struct {
							IsResolved githubv4.Boolean
						}
					} `graphql:"reviewThreads(first: 100)"`
					Repository struct {
						Name  githubv4.String
						Owner struct {
							Login githubv4.String
						}
					}
					Commits struct {
						Nodes []struct {
							Commit struct {
								StatusCheckRollup *struct {
									State    githubv4.String
									Contexts struct {
										Nodes []struct {
											Typename string `graphql:"__typename"`
											CheckRun struct {
												Name       githubv4.String
												Status     githubv4.String
												Conclusion githubv4.String
											} `graphql:"... on CheckRun"`
										}
									} `graphql:"contexts(first: 50)"`
								}
							}
						}
					} `graphql:"commits(last: 1)"`
				} `graphql:"... on PullRequest"`
			}
		} `graphql:"search(query: $query, type: ISSUE, first: 50)"`
	}

	variables := map[string]interface{}{
		"query": githubv4.String(sb.String()),
	}

	if err := client.Query(ctx, &q, variables); err != nil {
		return nil, err
	}

	var prs []PullRequest
	for _, node := range q.Search.Nodes {
		pr := node.PullRequest
		if string(pr.Title) == "" {
			continue
		}

		var checkStatus string
		var checkRuns []CheckRun
		if len(pr.Commits.Nodes) > 0 {
			rollup := pr.Commits.Nodes[0].Commit.StatusCheckRollup
			if rollup != nil {
				checkStatus = string(rollup.State)
				for _, ctxNode := range rollup.Contexts.Nodes {
					if ctxNode.Typename != "CheckRun" {
						continue
					}
					checkRuns = append(checkRuns, CheckRun{
						Name:       string(ctxNode.CheckRun.Name),
						Status:     string(ctxNode.CheckRun.Status),
						Conclusion: string(ctxNode.CheckRun.Conclusion),
					})
				}
			}
		}

		unresolved := countUnresolved(pr.ReviewThreads.Nodes, int(pr.ReviewThreads.TotalCount))

		prs = append(prs, PullRequest{
			ID:                fmt.Sprintf("%v", pr.ID),
			Number:            int(pr.Number),
			Title:             string(pr.Title),
			URL:               string(pr.URL),
			Repo:              string(pr.Repository.Name),
			Org:               string(pr.Repository.Owner.Login),
			Author:            string(pr.Author.Login),
			CreatedAt:         pr.CreatedAt.Time,
			UpdatedAt:         pr.UpdatedAt.Time,
			CheckStatus:       checkStatus,
			CheckRuns:         checkRuns,
			ReviewDecision:    string(pr.ReviewDecision),
			IsDraft:           bool(pr.IsDraft),
			TotalComments:     int(pr.TotalCommentsCount),
			UnresolvedThreads: unresolved,
			TotalThreads:      int(pr.ReviewThreads.TotalCount),
			Mergeable:         string(pr.Mergeable),
		})
	}

	return prs, nil
}

func fetchOrgPRs(ctx context.Context, client *githubv4.Client, orgs []string) ([]PullRequest, error) {
	var sb strings.Builder
	sb.WriteString("is:pr is:open archived:false sort:updated-desc")
	for _, org := range orgs {
		if org == "" {
			continue
		}
		sb.WriteString(" org:")
		sb.WriteString(org)
	}

	var allPRs []PullRequest
	var after *githubv4.String

	for {
		var q struct {
			Search struct {
				PageInfo struct {
					HasNextPage githubv4.Boolean
					EndCursor   githubv4.String
				}
				Nodes []struct {
					PullRequest struct {
						ID                 githubv4.ID
						Number             githubv4.Int
						Title              githubv4.String
						URL                githubv4.String
						IsDraft            githubv4.Boolean
						ReviewDecision     githubv4.String
						Mergeable          githubv4.String
						TotalCommentsCount githubv4.Int
						Author             struct {
							Login githubv4.String
						}
						CreatedAt githubv4.DateTime
						UpdatedAt githubv4.DateTime
						ReviewThreads struct {
							TotalCount githubv4.Int
						}
						Repository struct {
							Name  githubv4.String
							Owner struct {
								Login githubv4.String
							}
						}
						Commits struct {
							Nodes []struct {
								Commit struct {
									StatusCheckRollup *struct {
										State githubv4.String
									}
								}
							}
						} `graphql:"commits(last: 1)"`
					} `graphql:"... on PullRequest"`
				}
			} `graphql:"search(query: $query, type: ISSUE, first: 100, after: $after)"`
		}

		variables := map[string]interface{}{
			"query": githubv4.String(sb.String()),
			"after": after,
		}

		if err := client.Query(ctx, &q, variables); err != nil {
			return nil, err
		}

		for _, node := range q.Search.Nodes {
			pr := node.PullRequest
			if string(pr.Title) == "" {
				continue
			}

			var checkStatus string
			if len(pr.Commits.Nodes) > 0 {
				rollup := pr.Commits.Nodes[0].Commit.StatusCheckRollup
				if rollup != nil {
					checkStatus = string(rollup.State)
				}
			}

			allPRs = append(allPRs, PullRequest{
				ID:             fmt.Sprintf("%v", pr.ID),
				Number:         int(pr.Number),
				Title:          string(pr.Title),
				URL:            string(pr.URL),
				Repo:           string(pr.Repository.Name),
				Org:            string(pr.Repository.Owner.Login),
				Author:         string(pr.Author.Login),
				CreatedAt:      pr.CreatedAt.Time,
				UpdatedAt:      pr.UpdatedAt.Time,
				CheckStatus:    checkStatus,
				CheckRuns:      nil,
				ReviewDecision: string(pr.ReviewDecision),
				IsDraft:        bool(pr.IsDraft),
				TotalComments:  int(pr.TotalCommentsCount),
				TotalThreads:   int(pr.ReviewThreads.TotalCount),
				Mergeable:      string(pr.Mergeable),
			})
		}

		if !bool(q.Search.PageInfo.HasNextPage) {
			break
		}
		cursor := q.Search.PageInfo.EndCursor
		after = &cursor

		if len(allPRs) >= 1000 {
			break
		}
	}

	return allPRs, nil
}

func fetchCheckRuns(ctx context.Context, client *githubv4.Client, prID string) ([]CheckRun, error) {
	var q struct {
		Node struct {
			PullRequest struct {
				Commits struct {
					Nodes []struct {
						Commit struct {
							StatusCheckRollup *struct {
								Contexts struct {
									Nodes []struct {
										Typename string `graphql:"__typename"`
										CheckRun struct {
											Name       githubv4.String
											Status     githubv4.String
											Conclusion githubv4.String
										} `graphql:"... on CheckRun"`
									}
								} `graphql:"contexts(first: 50)"`
							}
						}
					}
				} `graphql:"commits(last: 1)"`
			} `graphql:"... on PullRequest"`
		} `graphql:"node(id: $id)"`
	}

	variables := map[string]interface{}{
		"id": githubv4.ID(prID),
	}

	if err := client.Query(ctx, &q, variables); err != nil {
		return nil, err
	}

	var runs []CheckRun
	pr := q.Node.PullRequest
	if len(pr.Commits.Nodes) > 0 {
		rollup := pr.Commits.Nodes[0].Commit.StatusCheckRollup
		if rollup != nil {
			for _, ctxNode := range rollup.Contexts.Nodes {
				if ctxNode.Typename != "CheckRun" {
					continue
				}
				runs = append(runs, CheckRun{
					Name:       string(ctxNode.CheckRun.Name),
					Status:     string(ctxNode.CheckRun.Status),
					Conclusion: string(ctxNode.CheckRun.Conclusion),
				})
			}
		}
	}

	return runs, nil
}

func closePR(ctx context.Context, client *githubv4.Client, prID string) error {
	var m struct {
		ClosePullRequest struct {
			PullRequest struct {
				State githubv4.String
			}
		} `graphql:"closePullRequest(input:$input)"`
	}
	input := githubv4.ClosePullRequestInput{
		PullRequestID: githubv4.ID(prID),
	}
	return client.Mutate(ctx, &m, input, nil)
}

func mergePR(ctx context.Context, client *githubv4.Client, prID string) error {
	var m struct {
		MergePullRequest struct {
			PullRequest struct {
				State githubv4.String
			}
		} `graphql:"mergePullRequest(input:$input)"`
	}
	squash := githubv4.PullRequestMergeMethodSquash
	input := githubv4.MergePullRequestInput{
		PullRequestID: githubv4.ID(prID),
		MergeMethod:   &squash,
	}
	return client.Mutate(ctx, &m, input, nil)
}

func approvePR(ctx context.Context, client *githubv4.Client, prID string) error {
	var m struct {
		AddPullRequestReview struct {
			PullRequestReview struct {
				State githubv4.String
			}
		} `graphql:"addPullRequestReview(input:$input)"`
	}
	event := githubv4.PullRequestReviewEventApprove
	input := githubv4.AddPullRequestReviewInput{
		PullRequestID: githubv4.ID(prID),
		Event:         &event,
	}
	return client.Mutate(ctx, &m, input, nil)
}

func addPRComment(ctx context.Context, client *githubv4.Client, subjectID, body string) error {
	var m struct {
		AddComment struct {
			CommentEdge struct {
				Node struct {
					ID githubv4.ID
				}
			}
		} `graphql:"addComment(input:$input)"`
	}
	input := githubv4.AddCommentInput{
		SubjectID: githubv4.ID(subjectID),
		Body:      githubv4.String(body),
	}
	return client.Mutate(ctx, &m, input, nil)
}
