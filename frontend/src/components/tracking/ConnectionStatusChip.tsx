interface ChipProps {
  state: 'connected' | 'reconnecting' | 'offline';
  className?: string;
}

export function ConnectionStatusChip({ state, className }: ChipProps) {
  const map = {
    connected: { dot: 'bg-green-500', text: 'Онлайн' },
    reconnecting: { dot: 'bg-amber-500 animate-pulse', text: 'Переподключение...' },
    offline: { dot: 'bg-red-500', text: 'Оффлайн' },
  } as const;
  const { dot, text } = map[state];
  return (
    <div
      role="status"
      aria-live="polite"
      className={`inline-flex items-center gap-1.5 text-xs px-2 py-1 rounded-full bg-white shadow border border-gray-200 ${className ?? ''}`}
    >
      <span className={`inline-block w-2 h-2 rounded-full ${dot}`} />
      <span className="text-gray-700">{text}</span>
    </div>
  );
}
