import { formatDistanceToNow, parseISO } from 'date-fns';
import { ru } from 'date-fns/locale';

export interface TimeAgoProps {
  iso: string;
  className?: string;
}

export function TimeAgo({ iso, className }: TimeAgoProps) {
  let text = '';
  try {
    text = formatDistanceToNow(parseISO(iso), { locale: ru, addSuffix: true });
  } catch {
    text = '';
  }
  return (
    <span className={className ?? 'text-xs text-gray-400 whitespace-nowrap'}>
      {text}
    </span>
  );
}
