export const IMAGE_CANVAS_GAP = 24
export const IMAGE_CANVAS_SNAP_THRESHOLD = 8
export const IMAGE_CANVAS_SNAP_RELEASE_THRESHOLD = 12

export interface CanvasLayoutNode {
  id: string
  x: number
  y: number
  width: number
  height: number
}

export interface CanvasSnapGuide {
  kind: 'alignment' | 'gap'
  x1: number
  y1: number
  x2: number
  y2: number
  label?: string
}

export interface CanvasSnapResult {
  dx: number
  dy: number
  guides: CanvasSnapGuide[]
  snappedX: boolean
  snappedY: boolean
}

interface SnapCandidate {
  delta: number
  priority: number
  guide: CanvasSnapGuide
}

export function canvasNodeBounds(nodes: CanvasLayoutNode[]): CanvasLayoutNode {
  if (!nodes.length) return { id: 'empty', x: 0, y: 0, width: 0, height: 0 }
  const left = Math.min(...nodes.map((node) => node.x))
  const top = Math.min(...nodes.map((node) => node.y))
  const right = Math.max(...nodes.map((node) => node.x + node.width))
  const bottom = Math.max(...nodes.map((node) => node.y + node.height))
  return { id: 'selection', x: left, y: top, width: right - left, height: bottom - top }
}

export function calculateCanvasSnap(
  movingNodes: CanvasLayoutNode[],
  stationaryNodes: CanvasLayoutNode[],
  zoom: number,
  active = { x: false, y: false },
): CanvasSnapResult {
  if (!movingNodes.length || !stationaryNodes.length) {
    return { dx: 0, dy: 0, guides: [], snappedX: false, snappedY: false }
  }
  const safeZoom = zoom > 0 ? zoom : 1
  const moving = canvasNodeBounds(movingNodes)
  const xThreshold = (active.x ? IMAGE_CANVAS_SNAP_RELEASE_THRESHOLD : IMAGE_CANVAS_SNAP_THRESHOLD) / safeZoom
  const yThreshold = (active.y ? IMAGE_CANVAS_SNAP_RELEASE_THRESHOLD : IMAGE_CANVAS_SNAP_THRESHOLD) / safeZoom
  const xCandidates: SnapCandidate[] = []
  const yCandidates: SnapCandidate[] = []

  for (const target of stationaryNodes) {
    addAlignmentCandidates(moving, target, xCandidates, yCandidates)
    addGapCandidates(moving, target, xCandidates, yCandidates, Math.max(xThreshold, yThreshold))
  }

  const bestX = bestCandidate(xCandidates, xThreshold)
  const bestY = bestCandidate(yCandidates, yThreshold)
  return {
    dx: bestX?.delta || 0,
    dy: bestY?.delta || 0,
    guides: [bestX?.guide, bestY?.guide].filter((guide): guide is CanvasSnapGuide => Boolean(guide)),
    snappedX: Boolean(bestX),
    snappedY: Boolean(bestY),
  }
}

function addAlignmentCandidates(
  moving: CanvasLayoutNode,
  target: CanvasLayoutNode,
  xCandidates: SnapCandidate[],
  yCandidates: SnapCandidate[],
) {
  const movingRight = moving.x + moving.width
  const movingBottom = moving.y + moving.height
  const targetRight = target.x + target.width
  const targetBottom = target.y + target.height
  const verticalStart = Math.min(moving.y, target.y)
  const verticalEnd = Math.max(movingBottom, targetBottom)
  const horizontalStart = Math.min(moving.x, target.x)
  const horizontalEnd = Math.max(movingRight, targetRight)

  xCandidates.push(
    alignmentCandidate(target.x - moving.x, 1, target.x, verticalStart, target.x, verticalEnd),
    alignmentCandidate(targetRight - movingRight, 1, targetRight, verticalStart, targetRight, verticalEnd),
    alignmentCandidate(
      target.x + target.width / 2 - (moving.x + moving.width / 2),
      2,
      target.x + target.width / 2,
      verticalStart,
      target.x + target.width / 2,
      verticalEnd,
    ),
  )
  yCandidates.push(
    alignmentCandidate(target.y - moving.y, 1, horizontalStart, target.y, horizontalEnd, target.y),
    alignmentCandidate(targetBottom - movingBottom, 1, horizontalStart, targetBottom, horizontalEnd, targetBottom),
    alignmentCandidate(
      target.y + target.height / 2 - (moving.y + moving.height / 2),
      2,
      horizontalStart,
      target.y + target.height / 2,
      horizontalEnd,
      target.y + target.height / 2,
    ),
  )
}

function addGapCandidates(
  moving: CanvasLayoutNode,
  target: CanvasLayoutNode,
  xCandidates: SnapCandidate[],
  yCandidates: SnapCandidate[],
  tolerance: number,
) {
  const movingRight = moving.x + moving.width
  const movingBottom = moving.y + moving.height
  const targetRight = target.x + target.width
  const targetBottom = target.y + target.height

  if (rangesOverlap(moving.y, movingBottom, target.y, targetBottom, tolerance)) {
    const guideY = overlapMidpoint(moving.y, movingBottom, target.y, targetBottom)
    xCandidates.push(
      gapCandidate(targetRight + IMAGE_CANVAS_GAP - moving.x, targetRight, guideY, targetRight + IMAGE_CANVAS_GAP, guideY),
      gapCandidate(target.x - IMAGE_CANVAS_GAP - movingRight, target.x - IMAGE_CANVAS_GAP, guideY, target.x, guideY),
    )
  }
  if (rangesOverlap(moving.x, movingRight, target.x, targetRight, tolerance)) {
    const guideX = overlapMidpoint(moving.x, movingRight, target.x, targetRight)
    yCandidates.push(
      gapCandidate(targetBottom + IMAGE_CANVAS_GAP - moving.y, guideX, targetBottom, guideX, targetBottom + IMAGE_CANVAS_GAP),
      gapCandidate(target.y - IMAGE_CANVAS_GAP - movingBottom, guideX, target.y - IMAGE_CANVAS_GAP, guideX, target.y),
    )
  }
}

function alignmentCandidate(delta: number, priority: number, x1: number, y1: number, x2: number, y2: number): SnapCandidate {
  return { delta, priority, guide: { kind: 'alignment', x1, y1, x2, y2 } }
}

function gapCandidate(delta: number, x1: number, y1: number, x2: number, y2: number): SnapCandidate {
  return { delta, priority: 0, guide: { kind: 'gap', x1, y1, x2, y2, label: String(IMAGE_CANVAS_GAP) } }
}

function bestCandidate(candidates: SnapCandidate[], threshold: number) {
  return candidates
    .filter((candidate) => Math.abs(candidate.delta) <= threshold)
    .sort((left, right) => Math.abs(left.delta) - Math.abs(right.delta) || left.priority - right.priority)[0]
}

function rangesOverlap(startA: number, endA: number, startB: number, endB: number, tolerance: number) {
  return startA <= endB + tolerance && startB <= endA + tolerance
}

function overlapMidpoint(startA: number, endA: number, startB: number, endB: number) {
  const start = Math.max(startA, startB)
  const end = Math.min(endA, endB)
  if (start <= end) return (start + end) / 2
  return (startA + endA + startB + endB) / 4
}
