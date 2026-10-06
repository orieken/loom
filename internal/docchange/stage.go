package docchange

import "github.com/orieken/loom/internal/genblock"

// stageWhole stages a file this tool owns completely.
func (p *planner) stageWhole(kind, target, content string) error {
	current, exists, err := readTarget(p.input.Root, target)
	if err != nil {
		return err
	}
	status := genblock.Replaced
	if !exists {
		status = genblock.Created
	}
	if exists && current == content {
		status = genblock.Unchanged
	}
	return p.record(kind, target, content, status, "")
}

// stageBlock stages a generated block inside a file a human may also edit.
func (p *planner) stageBlock(kind, target string, syntax genblock.Syntax, body string) error {
	current, exists, err := readTarget(p.input.Root, target)
	if err != nil {
		return err
	}
	content, status, reason := syntax.Upsert(current, exists, body)
	return p.record(kind, target, content, status, reason)
}

// stageEdit stages a planned source-documentation edit, or records why not.
func (p *planner) stageEdit(kind string, edit sourceEdit) error {
	if edit.skip != "" {
		return p.addArtifact(Artifact{Target: edit.target, Kind: kind, Action: ActionSkip, Reason: edit.skip})
	}
	return p.record(kind, edit.target, edit.content, edit.status, edit.reason)
}

// record turns a result into an artifact and, when something would be
// written, stages its final content.
func (p *planner) record(kind, target, content string, status genblock.Status, reason string) error {
	base, err := digestOfTarget(p.input.Root, target)
	if err != nil {
		return err
	}
	artifact := Artifact{Target: target, Kind: kind, BaseDigest: base}
	switch status {
	case genblock.Conflict:
		artifact.Action, artifact.Reason = ActionConflict, reason
	case genblock.Unchanged:
		artifact.Action = ActionUnchanged
	case genblock.Created:
		artifact.Action = ActionCreate
	default:
		artifact.Action = ActionUpdate
	}
	if artifact.Staged() {
		artifact.Digest = Digest([]byte(content))
		if err := stage(p.input.Root, p.change.ID, target, []byte(content)); err != nil {
			return err
		}
	}
	return p.addArtifact(artifact)
}

func (p *planner) addArtifact(artifact Artifact) error {
	p.change.Artifacts = append(p.change.Artifacts, artifact)
	return nil
}
