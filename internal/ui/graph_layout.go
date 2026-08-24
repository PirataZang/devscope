package ui

import (
	"math"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
)

// This file computes the commit graph's lane layout ourselves, instead of
// parsing git's own `--graph` ASCII output — porting the approach from
// trasta298/keifu (MIT), whose Rust implementation renders rounded curves
// (╭╮╰╯) rather than diagonals (/\). Git's `--graph` never emits those
// glyphs, so getting this look means computing lane assignments directly
// from each commit's parent hashes.

type cellType int

const (
	cellEmpty cellType = iota
	cellPipe
	cellCommit
	cellBranchRight // ╭ — a new lane starts here, continuing down
	cellBranchLeft  // ╮ — a new lane starts here, continuing down (to the left)
	cellMergeRight  // ╰ — a lane ends here, joining from the right
	cellMergeLeft   // ╯ — a lane ends here, joining from the left
	cellHorizontal  // ─
	cellHorizontalPipe
	cellTeeRight // ├
	cellTeeLeft  // ┤
	cellTeeUp    // ┴ — fork point: one commit is the base of multiple children
)

type graphCell struct {
	kind      cellType
	color     int
	pipeColor int // only cellHorizontalPipe: color of the pipe being crossed
}

type graphNode struct {
	commit     *collectors.DAGCommit // nil = connector-only row (fork point)
	lane       int
	colorIndex int
	cells      []graphCell
}

type graphLayout struct {
	nodes   []graphNode
	maxLane int
}

// lanePalette mirrors keifu's 11-color rotation via standard ANSI indices —
// bright variants read clearly against a dark background in any terminal
// theme, and index 9 (bright blue) is reserved for the main branch.
var lanePalette = []lipgloss.Color{
	lipgloss.Color("6"),  // cyan
	lipgloss.Color("2"),  // green
	lipgloss.Color("5"),  // magenta
	lipgloss.Color("3"),  // yellow
	lipgloss.Color("1"),  // red
	lipgloss.Color("14"), // bright cyan
	lipgloss.Color("10"), // bright green
	lipgloss.Color("13"), // bright magenta
	lipgloss.Color("11"), // bright yellow
	lipgloss.Color("12"), // bright blue — main branch
	lipgloss.Color("9"),  // bright red
}

const mainBranchColorIdx = 9

func graphLaneGlyphColor(idx int) lipgloss.Color {
	return lanePalette[idx%len(lanePalette)]
}

// colorAssigner picks a lane's color to minimize visual clashes with
// whatever's currently on screen — ported from keifu's penalty-based
// assignment (nearby lanes, recent rows, fork siblings, and overall usage
// all push a color's score up; the lowest-penalty color wins).
type colorAssigner struct {
	laneColors        []int // -1 = none
	laneLastColor     []int
	nextColorIndex    int
	reservedColors    map[int]bool
	recentAssignments []colorAssignment
	historyWindow     int
	currentRow        int
	currentForkColors map[int]bool
	colorUsageCount   [11]int
	mainLane          int // -1 = none
}

type colorAssignment struct{ row, lane, color int }

func newColorAssigner() *colorAssigner {
	return &colorAssigner{
		reservedColors:    map[int]bool{},
		historyWindow:     6,
		currentForkColors: map[int]bool{},
		mainLane:          -1,
	}
}

func (c *colorAssigner) isMainLane(lane int) bool { return c.mainLane == lane }
func (c *colorAssigner) mainColor() int           { return mainBranchColorIdx }
func (c *colorAssigner) reserveColor(idx int)     { c.reservedColors[idx] = true }

func (c *colorAssigner) ensureCapacity(lane int) {
	for len(c.laneColors) <= lane {
		c.laneColors = append(c.laneColors, -1)
		c.laneLastColor = append(c.laneLastColor, 0)
	}
}

func (c *colorAssigner) advanceRow() {
	c.currentRow++
	c.currentForkColors = map[int]bool{}
}

func (c *colorAssigner) beginFork() {
	c.currentForkColors = map[int]bool{}
}

func (c *colorAssigner) assignColorAdvanced(lane int, isForkSibling, useReserved bool) int {
	c.ensureCapacity(lane)
	var penalties [11]float64

	penalties[c.laneLastColor[lane]] += 10.0

	for otherLane, color := range c.laneColors {
		if color < 0 {
			continue
		}
		dist := math.Abs(float64(lane - otherLane))
		penalties[color] += 8.0 / (dist + 1.0)
	}

	for _, rec := range c.recentAssignments {
		rowDist := float64(c.currentRow - rec.row)
		if rowDist < 0 {
			rowDist = 0
		}
		laneDist := math.Abs(float64(lane - rec.lane))
		penalties[rec.color] += (4.0 / (rowDist + 1.0)) * (2.0 / (laneDist + 1.0))
	}

	if isForkSibling {
		for color := range c.currentForkColors {
			penalties[color] += 100.0
		}
	}

	maxUsage := 0
	for _, u := range c.colorUsageCount {
		if u > maxUsage {
			maxUsage = u
		}
	}
	if maxUsage > 0 {
		for color, count := range c.colorUsageCount {
			penalties[color] += (float64(count) / float64(maxUsage)) * 2.0
		}
	}

	bestColor := c.nextColorIndex
	bestPenalty := math.MaxFloat64
	for candidate := 0; candidate < len(lanePalette); candidate++ {
		colorIdx := (c.nextColorIndex + candidate) % len(lanePalette)
		if !useReserved && c.reservedColors[colorIdx] {
			continue
		}
		if penalties[colorIdx] < bestPenalty {
			bestPenalty = penalties[colorIdx]
			bestColor = colorIdx
		}
	}

	c.laneColors[lane] = bestColor
	c.laneLastColor[lane] = bestColor
	c.nextColorIndex = (bestColor + 1) % len(lanePalette)

	c.recentAssignments = append(c.recentAssignments, colorAssignment{c.currentRow, lane, bestColor})
	if len(c.recentAssignments) > c.historyWindow {
		c.recentAssignments = c.recentAssignments[1:]
	}
	c.colorUsageCount[bestColor]++
	if isForkSibling {
		c.currentForkColors[bestColor] = true
	}
	return bestColor
}

func (c *colorAssigner) assignColor(lane int) int             { return c.assignColorAdvanced(lane, false, false) }
func (c *colorAssigner) assignForkSiblingColor(lane int) int  { return c.assignColorAdvanced(lane, true, false) }

func (c *colorAssigner) assignMainColor(lane int) int {
	c.ensureCapacity(lane)
	color := mainBranchColorIdx
	c.laneColors[lane] = color
	c.laneLastColor[lane] = color
	c.reserveColor(color)
	c.mainLane = lane
	c.colorUsageCount[color]++
	return color
}

func (c *colorAssigner) continueLane(lane int) int {
	if c.mainLane == lane {
		return mainBranchColorIdx
	}
	c.ensureCapacity(lane)
	if c.laneColors[lane] >= 0 {
		return c.laneColors[lane]
	}
	return c.assignColor(lane)
}

func (c *colorAssigner) releaseLane(lane int) {
	if lane >= 0 && lane < len(c.laneColors) && c.mainLane != lane {
		c.laneColors[lane] = -1
	}
}

func findLane(lanes []string, hash string) int {
	for i, h := range lanes {
		if h == hash {
			return i
		}
	}
	return -1
}

func firstEmptyLane(lanes []string) int {
	for i, h := range lanes {
		if h == "" {
			return i
		}
	}
	return -1
}

// buildGraphLayout assigns every commit a lane and, for each row, a strip of
// cells describing exactly what to draw in every lane column (and the gap
// column between lanes) — this is the direct Go port of keifu's
// build_graph/build_row_cells_with_colors/build_fork_connector_cells.
func buildGraphLayout(commits []collectors.DAGCommit) graphLayout {
	if len(commits) == 0 {
		return graphLayout{}
	}

	hashKnown := make(map[string]bool, len(commits))
	for _, c := range commits {
		hashKnown[c.Hash] = true
	}

	parentChildren := make(map[string][]string)
	for _, c := range commits {
		for _, parentHash := range c.Parents {
			if hashKnown[parentHash] {
				parentChildren[parentHash] = append(parentChildren[parentHash], c.Hash)
			}
		}
	}
	forkPoints := make(map[string]bool)
	for parent, children := range parentChildren {
		if len(children) >= 2 {
			forkPoints[parent] = true
		}
	}

	var lanes []string
	var nodes []graphNode
	maxLane := 0

	ca := newColorAssigner()
	hashColorIndex := make(map[string]int)
	laneColorIndex := make(map[int]int)

	colorFor := func(lane int, hash string, fallback int) int {
		if c, ok := laneColorIndex[lane]; ok {
			return c
		}
		if c, ok := hashColorIndex[hash]; ok {
			return c
		}
		return fallback
	}

	for _, commit := range commits {
		ca.advanceRow()

		commitLaneOpt := findLane(lanes, commit.Hash)
		var lane int
		if commitLaneOpt >= 0 {
			lane = commitLaneOpt
		} else if empty := firstEmptyLane(lanes); empty >= 0 {
			lane = empty
		} else {
			lanes = append(lanes, "")
			lane = len(lanes) - 1
		}

		var forkLanes []int
		for i, h := range lanes {
			if h == commit.Hash {
				forkLanes = append(forkLanes, i)
			}
		}

		if len(forkLanes) >= 2 {
			mainLane := forkLanes[0]
			for _, l := range forkLanes {
				if l < mainLane {
					mainLane = l
				}
			}
			type mergingLane struct{ lane, color int }
			var mergingLanes []mergingLane
			for _, l := range forkLanes {
				if l != mainLane {
					mergingLanes = append(mergingLanes, mergingLane{l, colorFor(l, commit.Hash, l)})
				}
			}
			for _, ml := range mergingLanes {
				maxLane = maxInt(maxLane, ml.lane)
			}
			maxLane = maxInt(maxLane, mainLane)

			mainColor := colorFor(mainLane, commit.Hash, mainLane)
			pairs := make([][2]int, len(mergingLanes))
			for i, ml := range mergingLanes {
				pairs[i] = [2]int{ml.lane, ml.color}
			}
			cells := buildForkConnectorCells(mainLane, mainColor, pairs, lanes, hashColorIndex, laneColorIndex, maxLane)
			nodes = append(nodes, graphNode{commit: nil, lane: mainLane, colorIndex: mainColor, cells: cells})

			for _, ml := range mergingLanes {
				if ml.lane < len(lanes) {
					lanes[ml.lane] = ""
					ca.releaseLane(ml.lane)
					delete(laneColorIndex, ml.lane)
				}
			}
		}

		var commitColorIndex int
		if commitLaneOpt >= 0 {
			commitColorIndex = ca.continueLane(lane)
		} else if len(nodes) == 0 {
			commitColorIndex = ca.assignMainColor(lane)
		} else {
			commitColorIndex = ca.assignColor(lane)
		}
		hashColorIndex[commit.Hash] = commitColorIndex
		laneColorIndex[lane] = commitColorIndex

		if lane < len(lanes) {
			lanes[lane] = ""
		}

		type parentLane struct {
			hash        string
			lane        int
			wasExisting bool
			color       int
			alreadyShown bool
		}
		var parentLanes []parentLane
		var validParents []string
		for _, ph := range commit.Parents {
			if hashKnown[ph] {
				validParents = append(validParents, ph)
			}
		}

		var forkSiblingColor = -1
		if len(validParents) >= 2 {
			ca.beginFork()
		}

		alreadyShown := func(hash string) bool {
			for _, n := range nodes {
				if n.commit != nil && n.commit.Hash == hash {
					return true
				}
			}
			return false
		}

		for idx, parentHash := range validParents {
			existingLane := findLane(lanes, parentHash)
			shown := alreadyShown(parentHash)

			var pl int
			var wasExisting bool
			var color int
			switch {
			case existingLane >= 0 && idx == 0 && forkPoints[parentHash]:
				lanes[lane] = parentHash
				if ca.isMainLane(lane) {
					color = ca.mainColor()
				} else {
					color = commitColorIndex
				}
				forkSiblingColor = color
				laneColorIndex[lane] = color
				pl, wasExisting = lane, false
			case existingLane >= 0:
				color = colorFor(existingLane, parentHash, existingLane)
				pl, wasExisting = existingLane, true
			case idx == 0:
				lanes[lane] = parentHash
				hashColorIndex[parentHash] = commitColorIndex
				pl, wasExisting, color = lane, false, commitColorIndex
			default:
				newLane := firstEmptyLane(lanes)
				if newLane < 0 {
					lanes = append(lanes, "")
					newLane = len(lanes) - 1
				}
				lanes[newLane] = parentHash
				color = ca.assignForkSiblingColor(newLane)
				hashColorIndex[parentHash] = color
				laneColorIndex[newLane] = color
				pl, wasExisting = newLane, false
			}
			parentLanes = append(parentLanes, parentLane{parentHash, pl, wasExisting, color, shown})
		}

		finalColor := commitColorIndex
		if forkSiblingColor >= 0 {
			finalColor = forkSiblingColor
		}

		maxLane = maxInt(maxLane, lane)
		for _, pl := range parentLanes {
			maxLane = maxInt(maxLane, pl.lane)
		}

		rowParents := make([]rowParentInfo, len(parentLanes))
		for i, pl := range parentLanes {
			rowParents[i] = rowParentInfo{pl.lane, pl.wasExisting, pl.color, pl.alreadyShown}
		}
		cells := buildRowCells(lane, finalColor, rowParents, lanes, hashColorIndex, laneColorIndex, maxLane)

		commitCopy := commit
		nodes = append(nodes, graphNode{commit: &commitCopy, lane: lane, colorIndex: finalColor, cells: cells})

		// Lane merge: a parent is already tracked on a different lane.
		var mergeParentLane = -1
		var haveMerge bool
		for _, pl := range parentLanes {
			if pl.wasExisting && pl.lane != lane {
				mergeParentLane, haveMerge = pl.lane, true
				break
			}
		}
		if haveMerge {
			mainLane, endingLane := mergeParentLane, lane
			if lane < mergeParentLane {
				mainLane, endingLane = lane, mergeParentLane
			}
			var endingHash string
			if endingLane < len(lanes) {
				endingHash = lanes[endingLane]
			}
			endingShown := endingHash == "" || alreadyShown(endingHash)
			continuesDown := endingHash != "" && !endingShown

			if endingLane < len(lanes) {
				firstParentOnEnding := len(parentLanes) > 0 && parentLanes[0].lane == endingLane
				if !firstParentOnEnding && !continuesDown {
					if h := lanes[endingLane]; h != "" && mainLane < len(lanes) && lanes[mainLane] == "" {
						lanes[mainLane] = h
					}
					lanes[endingLane] = ""
					ca.releaseLane(endingLane)
					delete(laneColorIndex, endingLane)
				}
			}
		}
	}

	return graphLayout{nodes: nodes, maxLane: maxLane}
}

type rowParentInfo struct {
	lane         int
	wasExisting  bool
	color        int
	alreadyShown bool
}

func buildRowCells(commitLane, commitColor int, parents []rowParentInfo, activeLanes []string, hashColorIndex map[string]int, laneColorIndex map[int]int, maxLane int) []graphCell {
	cells := make([]graphCell, (maxLane+1)*2)

	colorFor := func(lane int, hash string, fallback int) int {
		if c, ok := laneColorIndex[lane]; ok {
			return c
		}
		if c, ok := hashColorIndex[hash]; ok {
			return c
		}
		return fallback
	}

	for laneIdx, hash := range activeLanes {
		if hash == "" || laneIdx == commitLane {
			continue
		}
		if idx := laneIdx * 2; idx < len(cells) {
			cells[idx] = graphCell{kind: cellPipe, color: colorFor(laneIdx, hash, laneIdx)}
		}
	}

	if idx := commitLane * 2; idx < len(cells) {
		cells[idx] = graphCell{kind: cellCommit, color: commitColor}
	}

	for _, p := range parents {
		if p.lane == commitLane {
			continue
		}
		if p.lane > commitLane {
			for col := commitLane*2 + 1; col < p.lane*2; col++ {
				if col >= len(cells) {
					continue
				}
				if cells[col].kind == cellPipe {
					cells[col] = graphCell{kind: cellHorizontalPipe, color: p.color, pipeColor: cells[col].color}
				} else if cells[col].kind == cellEmpty {
					cells[col] = graphCell{kind: cellHorizontal, color: p.color}
				}
			}
			if end := p.lane * 2; end < len(cells) {
				switch {
				case p.wasExisting && p.alreadyShown:
					cells[end] = graphCell{kind: cellMergeLeft, color: p.color}
				case p.wasExisting:
					cells[end] = graphCell{kind: cellTeeLeft, color: p.color}
				default:
					cells[end] = graphCell{kind: cellBranchLeft, color: p.color}
				}
			}
		} else {
			for col := p.lane*2 + 1; col < commitLane*2; col++ {
				if col >= len(cells) {
					continue
				}
				if cells[col].kind == cellPipe {
					cells[col] = graphCell{kind: cellHorizontalPipe, color: p.color, pipeColor: cells[col].color}
				} else if cells[col].kind == cellEmpty {
					cells[col] = graphCell{kind: cellHorizontal, color: p.color}
				}
			}
			if start := p.lane * 2; start < len(cells) {
				switch {
				case p.wasExisting && p.alreadyShown:
					cells[start] = graphCell{kind: cellMergeRight, color: p.color}
				case p.wasExisting:
					cells[start] = graphCell{kind: cellTeeRight, color: p.color}
				default:
					cells[start] = graphCell{kind: cellBranchRight, color: p.color}
				}
			}
		}
	}

	return cells
}

// buildForkConnectorCells draws a connector-only row (no commit) joining
// several lanes that all point at the same fork-point commit, e.g. "├─┴─╯".
func buildForkConnectorCells(mainLane, mainColor int, mergingLanes [][2]int, activeLanes []string, hashColorIndex map[string]int, laneColorIndex map[int]int, maxLane int) []graphCell {
	cells := make([]graphCell, (maxLane+1)*2)

	mergingSet := make(map[int]bool, len(mergingLanes))
	for _, ml := range mergingLanes {
		mergingSet[ml[0]] = true
	}

	if idx := mainLane * 2; idx < len(cells) {
		cells[idx] = graphCell{kind: cellTeeRight, color: mainColor}
	}

	colorFor := func(lane int, hash string, fallback int) int {
		if c, ok := laneColorIndex[lane]; ok {
			return c
		}
		if c, ok := hashColorIndex[hash]; ok {
			return c
		}
		return fallback
	}

	for laneIdx, hash := range activeLanes {
		if hash == "" || laneIdx == mainLane || mergingSet[laneIdx] {
			continue
		}
		if idx := laneIdx * 2; idx < len(cells) {
			cells[idx] = graphCell{kind: cellPipe, color: colorFor(laneIdx, hash, laneIdx)}
		}
	}

	rightmost := mainLane
	for _, ml := range mergingLanes {
		if ml[0] > rightmost {
			rightmost = ml[0]
		}
	}

	for _, ml := range mergingLanes {
		mergeLane, mergeColor := ml[0], ml[1]
		for col := mainLane*2 + 1; col < mergeLane*2; col++ {
			if col >= len(cells) {
				continue
			}
			switch cells[col].kind {
			case cellPipe:
				cells[col] = graphCell{kind: cellHorizontalPipe, color: mergeColor, pipeColor: cells[col].color}
			case cellEmpty, cellHorizontal:
				cells[col] = graphCell{kind: cellHorizontal, color: mergeColor}
			}
		}
		if end := mergeLane * 2; end < len(cells) {
			if mergeLane == rightmost {
				cells[end] = graphCell{kind: cellMergeLeft, color: mergeColor}
			} else {
				cells[end] = graphCell{kind: cellTeeUp, color: mergeColor}
			}
		}
	}

	return cells
}
