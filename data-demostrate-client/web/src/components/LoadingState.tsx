export function LoadingState() {
  return (
    <div className="loading-state">
      <div className="skeleton skeleton-hero" />
      <div className="skeleton-grid">
        {Array.from({ length: 6 }).map((_, index) => (
          <div key={index} className="skeleton skeleton-metric" />
        ))}
      </div>
      <div className="skeleton-grid skeleton-grid-wide">
        <div className="skeleton skeleton-chart" />
        <div className="skeleton skeleton-chart" />
      </div>
    </div>
  )
}
