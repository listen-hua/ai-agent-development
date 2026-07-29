import { describe, expect, it } from 'vitest'
import {
  calculateCanvasSnap,
  canvasNodeBounds,
  IMAGE_CANVAS_GAP,
} from './imageCanvasLayout'

const node = (id: string, x: number, y: number, width = 100, height = 100) => ({
  id, x, y, width, height,
})

describe('image canvas smart snapping', () => {
  it('snaps matching edges and centers independently', () => {
    const left = calculateCanvasSnap([node('moving', 7, 220)], [node('target', 0, 0)], 1)
    expect(left.dx).toBe(-7)
    expect(left.snappedX).toBe(true)
    expect(left.guides[0]).toMatchObject({ kind: 'alignment', x1: 0, x2: 0 })

    const center = calculateCanvasSnap([node('moving', 304, 6)], [node('target', 300, 0)], 1)
    expect(center.dx).toBe(-4)
    expect(center.dy).toBe(-6)
    expect(center.snappedX).toBe(true)
    expect(center.snappedY).toBe(true)
  })

  it('prefers a 24-unit gap when candidates have the same distance', () => {
    const result = calculateCanvasSnap(
      [node('moving', 226, 0)],
      [node('target', 100, 0)],
      1,
    )

    expect(result.dx).toBe(-2)
    expect(result.guides.find((guide) => guide.kind === 'gap')).toMatchObject({
      kind: 'gap',
      x1: 200,
      x2: 200 + IMAGE_CANVAS_GAP,
      label: '24',
    })
  })

  it('keeps the trigger distance visually stable across zoom levels', () => {
    const target = [node('target', 0, 0)]

    expect(calculateCanvasSnap([node('moving', 15, 200)], target, 0.5).snappedX).toBe(true)
    expect(calculateCanvasSnap([node('moving', 9, 200)], target, 1).snappedX).toBe(false)
    expect(calculateCanvasSnap([node('moving', 5, 200)], target, 2).snappedX).toBe(false)
  })

  it('uses the wider release distance only while an axis is snapped', () => {
    const target = [node('target', 0, 0)]

    expect(calculateCanvasSnap([node('moving', 10, 200)], target, 1).snappedX).toBe(false)
    expect(calculateCanvasSnap([node('moving', 10, 200)], target, 1, { x: true, y: false }).snappedX).toBe(true)
  })

  it('snaps a multi-selection by its group bounds', () => {
    const moving = [node('one', 230, 10), node('two', 354, 10)]
    const result = calculateCanvasSnap(moving, [node('target', 0, 0, 200, 100)], 1)

    expect(canvasNodeBounds(moving)).toMatchObject({ x: 230, width: 224 })
    expect(result.dx).toBe(-6)
    expect(result.snappedX).toBe(true)
  })

  it('does not snap outside the threshold', () => {
    expect(calculateCanvasSnap(
      [node('moving', 40, 260)],
      [node('target', 0, 0)],
      1,
    )).toMatchObject({ dx: 0, dy: 0, snappedX: false, snappedY: false })
  })
})
