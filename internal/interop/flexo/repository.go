package flexo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/interop/reposync"
)

// Repository is one project branch as a sync's repository: read as a graph,
// written through the service's commit path so every element keeps its id.
type Repository struct {
	client  *Client
	project string
	branch  string
}

// Repository addresses one project branch for a sync.
func (c *Client) Repository(project, branch string) *Repository {
	return &Repository{client: c, project: project, branch: branch}
}

// Graph reads the branch as it stands.
func (r *Repository) Graph(ctx context.Context) (*rdf.Graph, error) {
	return r.client.BranchGraph(ctx, r.project, r.branch)
}

// GraphAt reads the branch as one earlier commit left it: the last-seen graph
// a diff needs to tell a repository change from its own.
func (r *Repository) GraphAt(ctx context.Context, commit string) (*rdf.Graph, error) {
	return r.client.CommitGraph(ctx, r.project, commit)
}

// Commit writes one batch as one SysML v2 commit. Creates and updates send
// the element whole under its identity; deletes send a null payload.
func (r *Repository) Commit(ctx context.Context, changes []reposync.ElementChange, message string) (string, error) {
	body, err := commitRequest(changes, message)
	if err != nil {
		return "", err
	}
	commit, err := r.client.PostChanges(ctx, r.project, r.branch, body)
	if err != nil {
		return "", err
	}
	return commit.ID, nil
}

// commitRequest encodes a batch as the service's CommitRequest.
func commitRequest(changes []reposync.ElementChange, message string) ([]byte, error) {
	versions := make([]map[string]any, 0, len(changes))
	for _, change := range changes {
		version := map[string]any{
			"@type":    "DataVersion",
			"identity": map[string]string{"@id": change.ID},
			"payload":  nil,
		}
		switch change.Kind {
		case reposync.KindDelete:
		case reposync.KindCreate, reposync.KindUpdate:
			content, err := payload(change)
			if err != nil {
				return nil, fmt.Errorf("%s %s: %w", change.Kind, change.ID, err)
			}
			version["payload"] = content
		default:
			return nil, fmt.Errorf("%s %s: the service has no commit for a %s", change.Kind, change.ID, change.Kind)
		}
		versions = append(versions, version)
	}
	request := map[string]any{"@type": "Commit", "change": versions}
	if message != "" {
		request["description"] = message
	}
	return json.Marshal(request)
}

// ErrUntyped is an element with no single sysml: metaclass, which the
// service's payload cannot state.
var ErrUntyped = errors.New("the element has no single SysML metaclass")

// payload renders an element's content as the service's JSON: @id and @type,
// then each sysml: property as a value or, when multi-valued, an array.
func payload(change reposync.ElementChange) (map[string]json.RawMessage, error) {
	out := map[string]json.RawMessage{}
	values := map[string][]json.RawMessage{}
	metaclass := ""
	for _, triple := range change.Content {
		predicate := triple.Predicate.Value
		if predicate == rdf.RDFType {
			name, ok := strings.CutPrefix(triple.Object.Value, rdf.SysML)
			if !ok || !triple.Object.IsIRI() || metaclass != "" {
				return nil, ErrUntyped
			}
			metaclass = name
			continue
		}
		name, ok := strings.CutPrefix(predicate, rdf.SysML)
		if !ok {
			return nil, &UnrepresentableError{Term: triple.Predicate, Reason: "the service stores sysml: properties only"}
		}
		value, err := jsonValue(triple.Object)
		if err != nil {
			return nil, err
		}
		values[name] = append(values[name], value)
	}
	if metaclass == "" {
		return nil, ErrUntyped
	}
	for name, list := range values {
		if len(list) == 1 {
			out[name] = list[0]
			continue
		}
		array, err := json.Marshal(list)
		if err != nil {
			return nil, err
		}
		out[name] = array
	}
	id, err := json.Marshal(change.ID)
	if err != nil {
		return nil, err
	}
	out["@id"] = id
	if out["@type"], err = json.Marshal(metaclass); err != nil {
		return nil, err
	}
	return out, nil
}

// jsonValue spells one carried term as the service reads it back from JSON.
func jsonValue(term rdf.Term) (json.RawMessage, error) {
	carried, err := carryTerm(term)
	if err != nil {
		return nil, err
	}
	if carried.IsIRI() {
		if carried.Value == rdfNil {
			return json.RawMessage("null"), nil
		}
		return json.Marshal(map[string]string{"@id": rdf.LocalName(carried.Value)})
	}
	switch carried.Datatype {
	case rdf.XSD + "boolean", rdf.XSD + "integer", rdf.XSD + "decimal":
		return json.RawMessage(carried.Value), nil
	}
	return json.Marshal(carried.Value)
}
