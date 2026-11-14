package graph

import (
	"github.com/wricardo/OneTDraw-Solver/game/engine"
	"github.com/wricardo/OneTDraw-Solver/game/service"
	"github.com/wricardo/OneTDraw-Solver/graph/model"
)

// toNode converts an engine node.
func toNode(n engine.Node) *model.Node {
	return &model.Node{ID: n.ID, Level: n.Level, X: n.Pos.X, Y: n.Pos.Y}
}

func toNodes(ns []engine.Node) []*model.Node {
	out := make([]*model.Node, len(ns))
	for i, n := range ns {
		out[i] = toNode(n)
	}
	return out
}

// toEdge converts an engine edge. index is its position in Map.Edges, which is
// also the index of State.usage.
func toEdge(index int, e engine.Edge) *model.Edge {
	me := &model.Edge{Index: index, A: e.A, B: e.B, Count: e.Count, OneWay: e.Directed()}
	if e.Directed() {
		from, to := e.Dir.From, e.Dir.To
		me.From = &from
		me.To = &to
	}
	return me
}

func toEdges(es []engine.Edge) []*model.Edge {
	out := make([]*model.Edge, len(es))
	for i, e := range es {
		out[i] = toEdge(i, e)
	}
	return out
}

func toAnalysis(a engine.Analysis) *model.Analysis {
	return &model.Analysis{
		NodeCount: a.NodeCount, EdgeCount: a.EdgeCount, StepCount: a.StepCount,
		Connected: a.Connected, Directional: a.Directional,
		OddNodes: append([]int{}, a.OddNodes...), StartNodes: append([]int{}, a.StartNodes...),
		Verdict: string(a.Verdict), Message: a.Message(),
	}
}

// toMap converts a map to the public catalog entry.
func toMap(m *engine.Map) *model.Map {
	if m == nil {
		return nil
	}
	var par *int
	if m.Par >= 0 {
		p := m.Par
		par = &p
	}
	return &model.Map{
		ID: m.ID, Name: m.Name, Description: m.Description, Difficulty: m.Difficulty,
		NodeCount: len(m.Nodes), EdgeCount: len(m.Edges), StepCount: m.StepCount,
		LevelCount: m.LevelCount(), MaxMoves: m.MaxMoves, ParMoves: par,
		Analysis: toAnalysis(m.Analysis), Nodes: toNodes(m.Nodes), Edges: toEdges(m.Edges),
	}
}

// toAdminMap converts a map to the editor's full form.
func toAdminMap(m *engine.Map) *model.AdminMap {
	if m == nil {
		return nil
	}
	pub := toMap(m)
	var par *int
	if m.Par >= 0 {
		p := m.Par
		par = &p
	}
	return &model.AdminMap{
		ID: m.ID, Name: m.Name, Description: m.Description, Difficulty: m.Difficulty,
		NodeCount: pub.NodeCount, EdgeCount: pub.EdgeCount, StepCount: pub.StepCount,
		LevelCount: pub.LevelCount, MaxMoves: m.MaxMoves, ParMoves: par,
		Analysis: pub.Analysis, Nodes: pub.Nodes, Edges: pub.Edges,
		Config: toMapConfig(m.Config()),
	}
}

func toMapPoint(p engine.PointConfig, m *engine.Map) *model.MapPoint {
	mp := &model.MapPoint{Point: p.Point, Level: p.Level, Hidden: p.Hidden()}
	if n, ok := m.Node(p.Point); ok {
		mp.X, mp.Y = n.Pos.X, n.Pos.Y
	}
	return mp
}

// toMapConfig converts the stored JSON form.
func toMapConfig(cfg engine.MapConfig) *model.MapConfig {
	mc := &model.MapConfig{
		ID: cfg.ID, Name: cfg.Name, Description: cfg.Description, Difficulty: cfg.Difficulty,
		MaxMoves: cfg.MaxMoves, Points: []*model.MapPoint{}, Edges: []*model.ConfigEdge{},
	}
	parsed, _ := engine.NewMap(cfg)
	for _, p := range cfg.Points {
		var mp *model.MapPoint
		if parsed != nil {
			mp = toMapPoint(p, parsed)
		} else {
			mp = &model.MapPoint{Point: p.Point, Level: p.Level, Hidden: p.Hidden()}
		}
		mc.Points = append(mc.Points, mp)
	}
	for _, e := range cfg.Edges {
		ce := &model.ConfigEdge{PointA: e.PointA, PointB: e.PointB, Count: e.Count}
		if ce.Count == 0 {
			ce.Count = 1
		}
		if e.Direction != nil {
			from, to := e.Direction.From, e.Direction.To
			ce.From, ce.To = &from, &to
		}
		mc.Edges = append(mc.Edges, ce)
	}
	return mc
}

// fromMapInput converts a GraphQL MapInput to an engine.MapConfig.
func fromMapInput(in model.MapInput) engine.MapConfig {
	cfg := engine.MapConfig{ID: in.ID, Name: in.Name, Edges: []engine.EdgeConfig{}}
	if in.Description != nil {
		cfg.Description = *in.Description
	}
	if in.Difficulty != nil {
		cfg.Difficulty = *in.Difficulty
	}
	if in.MaxMoves != nil {
		cfg.MaxMoves = *in.MaxMoves
	}
	for _, p := range in.Points {
		level := 0
		if p.Level != nil {
			level = *p.Level
		}
		cfg.Points = append(cfg.Points, engine.NewPointConfig(p.Point, level, p.Hide != nil && *p.Hide))
	}
	for _, e := range in.Edges {
		ec := engine.EdgeConfig{PointA: e.PointA, PointB: e.PointB}
		if e.Count != nil {
			ec.Count = *e.Count
		}
		if e.Direction != nil {
			ec.Direction = &engine.Direction{
				From: e.Direction.From, To: e.Direction.To, Unidirectional: e.Direction.Unidirectional,
			}
		}
		cfg.Edges = append(cfg.Edges, ec)
	}
	return cfg
}

func toGameState(s engine.State) *model.GameState {
	var pos *int
	if s.Pos >= 0 {
		p := s.Pos
		pos = &p
	}
	return &model.GameState{
		Pos: pos, Status: model.Status(s.Status), LossReason: toLossReason(s.LossReason),
		Moves: s.Moves, Blocked: s.Blocked, Remaining: s.Remaining,
		Path: append([]int{}, s.Path...), Usage: s.Usage(),
	}
}

func toLossReason(r engine.LossReason) *model.LossReason {
	if r == engine.LossNone {
		return nil
	}
	lr := model.LossReason(r)
	return &lr
}

func toStepEvent(ev *engine.StepEvent) *model.StepEvent {
	if ev == nil {
		return nil
	}
	var from, to *int
	if ev.From >= 0 {
		f := ev.From
		from = &f
	}
	if ev.To >= 0 {
		t := ev.To
		to = &t
	}
	var edge *int
	if ev.Edge >= 0 {
		e := ev.Edge
		edge = &e
	}
	var reason *string
	if ev.Reason != "" {
		r := ev.Reason
		reason = &r
	}
	return &model.StepEvent{
		Move: ev.Move, Node: ev.Node, From: from, To: to, Kind: model.StepKind(ev.Kind),
		Edge: edge, Blocked: ev.Blocked, Reason: reason,
		Status: model.Status(ev.Status), LossReason: toLossReason(ev.LossReason),
	}
}

func toPlan(p *service.Plan) *model.Plan {
	if p == nil {
		return nil
	}
	return &model.Plan{Start: p.Start, Steps: append([]int{}, p.Steps...), Found: p.Found, ComputedAt: p.At}
}

// toSession converts an immutable session snapshot (a Clone).
func toSession(s *service.Session) *model.Session {
	if s == nil {
		return nil
	}
	return &model.Session{
		ID: s.ID, DisplayName: s.DisplayName, MapID: s.MapID,
		Map:   toMap(s.Map),
		State: toGameState(s.State),
		Seq:   int(s.Seq), CreatedAt: s.CreatedAt, LastActionAt: s.LastActionAt,
		Attempts: s.Attempts, BestMoves: s.BestMoves,
		LastEvent: toStepEvent(s.LastEvent), LastPlan: toPlan(s.LastPlan),
		AutoPlay: s.AutoPlay,
	}
}

func toSessionUpdate(s *service.Session) *model.SessionUpdate {
	return &model.SessionUpdate{Seq: int(s.Seq), Session: toSession(s), Event: toStepEvent(s.LastEvent)}
}
