import { useState, useCallback, type ReactNode } from 'react';
import { ChevronDown, ChevronUp } from 'lucide-react';
import { cn } from '@/lib/utils';

interface TrackingBottomSheetProps {
  children: ReactNode;
  defaultExpanded?: boolean;
  onToggle?: (expanded: boolean) => void;
  className?: string;
}

export function TrackingBottomSheet({
  children,
  defaultExpanded = true,
  onToggle,
  className,
}: TrackingBottomSheetProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  const toggle = useCallback(() => {
    setExpanded((prev) => {
      const next = !prev;
      onToggle?.(next);
      return next;
    });
  }, [onToggle]);

  return (
    <div
      className={cn(
        'fixed bottom-0 left-0 right-0 bg-white rounded-t-2xl shadow-2xl border-t border-gray-200',
        'transition-transform duration-200 ease-out',
        expanded ? 'translate-y-0' : 'translate-y-[calc(100%-60px)]',
        className,
      )}
      style={{ zIndex: 1000 }}
      data-state={expanded ? 'expanded' : 'collapsed'}
    >
      <button
        onClick={toggle}
        className="w-full py-3 flex flex-col items-center justify-center gap-1 cursor-pointer"
        aria-label={expanded ? 'Свернуть детали' : 'Развернуть детали'}
        aria-expanded={expanded}
      >
        <div className="w-10 h-1 bg-gray-300 rounded-full" />
        {expanded ? (
          <ChevronDown className="h-4 w-4 text-gray-400" aria-hidden="true" />
        ) : (
          <ChevronUp className="h-4 w-4 text-gray-400" aria-hidden="true" />
        )}
      </button>
      <div className="px-4 pb-6">{children}</div>
    </div>
  );
}
