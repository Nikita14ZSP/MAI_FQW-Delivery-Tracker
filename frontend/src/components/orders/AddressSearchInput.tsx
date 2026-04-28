import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Loader2, MapPin } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { searchAddress, type NominatimResult } from '@/lib/api/nominatim';

export interface AddressSelection {
  lat: number;
  lon: number;
  displayName: string;
}

export interface AddressSearchInputProps {
  onSelect: (selection: AddressSelection) => void;
  initialValue?: string;
}

export function AddressSearchInput({
  onSelect,
  initialValue = '',
}: AddressSearchInputProps) {
  const [value, setValue] = useState(initialValue);
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(value, 500);

  const { data, isFetching } = useQuery({
    queryKey: ['nominatim-search', debounced],
    queryFn: () => searchAddress(debounced),
    enabled: debounced.trim().length >= 3,
    staleTime: 60_000,
  });

  const results: NominatimResult[] = data ?? [];

  return (
    <div className="relative w-full">
      <Input
        type="text"
        placeholder="Поиск адреса в Москве..."
        value={value}
        onChange={(e) => {
          setValue(e.target.value);
          setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        aria-label="Адрес доставки"
        autoComplete="off"
      />
      {isFetching && (
        <Loader2
          className="absolute right-3 top-3 h-3.5 w-3.5 animate-spin text-gray-400"
          aria-hidden="true"
        />
      )}
      {open && debounced.trim().length >= 3 && !isFetching && (
        <div
          className="absolute top-full left-0 right-0 z-[1000] mt-1 max-h-72 overflow-y-auto rounded-lg border border-gray-200 bg-white shadow-md"
          role="listbox"
        >
          {results.length === 0 ? (
            <div className="px-3 py-2 text-sm text-gray-400">
              Адрес не найден
            </div>
          ) : (
            results.slice(0, 5).map((r) => (
              <button
                key={r.place_id}
                type="button"
                role="option"
                onClick={() => {
                  onSelect({
                    lat: parseFloat(r.lat),
                    lon: parseFloat(r.lon),
                    displayName: r.display_name,
                  });
                  setValue(r.display_name);
                  setOpen(false);
                }}
                className="flex w-full items-start gap-2 px-3 py-2 text-left text-sm hover:bg-gray-50"
              >
                <MapPin className="mt-0.5 h-3.5 w-3.5 flex-shrink-0 text-gray-400" />
                <span className="truncate">{r.display_name.slice(0, 60)}</span>
              </button>
            ))
          )}
        </div>
      )}
    </div>
  );
}
