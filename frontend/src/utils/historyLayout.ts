export function captureResultsHeight(measuredHeight: number) {
  return Math.ceil(measuredHeight)
}

export function preservedResultsStyle(minHeight: number, active: boolean) {
  return active && minHeight > 0 ? { minHeight: `${minHeight}px` } : undefined
}
