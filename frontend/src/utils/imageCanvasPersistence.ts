import type { ImageCanvas, ImageCanvasNode, ImageViewport } from '@/types/image-agent'

export interface ImageCanvasLayout {
  viewport: ImageViewport
  nodes: ImageCanvasNode[]
}

const LAYOUT_EPSILON = 0.01

export function cloneCanvasLayout(value: ImageCanvasLayout): ImageCanvasLayout {
  return {
    viewport: { ...(value.viewport || { x: 0, y: 0, zoom: 1 }) },
    nodes: (value.nodes || []).map((node) => ({ ...node })),
  }
}

export function canvasLayoutFromCanvas(canvas: ImageCanvas): ImageCanvasLayout {
  return cloneCanvasLayout(canvas)
}

export function sameCanvasLayout(left: ImageCanvasLayout, right: ImageCanvasLayout): boolean {
  const leftNodes = left.nodes || []
  const rightValues = right.nodes || []
  if (!sameViewport(left.viewport, right.viewport) || leftNodes.length !== rightValues.length) return false
  const rightNodes = new Map(rightValues.map((node) => [node.id, node]))
  return leftNodes.every((node) => {
    const other = rightNodes.get(node.id)
    return Boolean(other)
      && near(node.x, other!.x)
      && near(node.y, other!.y)
      && near(node.width, other!.width)
      && near(node.height, other!.height)
      && node.z_index === other!.z_index
  })
}

export function applyLocalCanvasLayout(canvas: ImageCanvas, layout: ImageCanvasLayout): ImageCanvas {
  const layoutNodes = new Map((layout.nodes || []).map((node) => [node.id, node]))
  return {
    ...canvas,
    viewport: { ...layout.viewport },
    nodes: (canvas.nodes || []).map((node) => {
      const local = layoutNodes.get(node.id)
      return local
        ? {
            ...node,
            x: local.x,
            y: local.y,
            width: local.width,
            height: local.height,
            z_index: local.z_index,
          }
        : node
    }),
  }
}

export function mergeRemoteCanvasWithLocalLayout(remote: ImageCanvas, local: ImageCanvas): ImageCanvas {
  const localNodes = new Map((local.nodes || []).map((node) => [node.id, node]))
  return {
    ...remote,
    viewport: { ...local.viewport },
    nodes: (remote.nodes || []).map((node) => {
      const current = localNodes.get(node.id)
      return current
        ? { ...node, x: current.x, y: current.y, z_index: current.z_index }
        : node
    }),
  }
}

function sameViewport(left: ImageViewport, right: ImageViewport) {
	left ||= { x: 0, y: 0, zoom: 1 }
	right ||= { x: 0, y: 0, zoom: 1 }
  return near(left.x, right.x) && near(left.y, right.y) && near(left.zoom, right.zoom)
}

function near(left: number, right: number) {
  return Math.abs(left - right) <= LAYOUT_EPSILON
}
