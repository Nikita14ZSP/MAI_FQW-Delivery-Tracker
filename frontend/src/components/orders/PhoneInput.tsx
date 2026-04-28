import { forwardRef } from 'react';
import { Input } from '@/components/ui/input';

export type PhoneInputProps = React.InputHTMLAttributes<HTMLInputElement>;

export const PhoneInput = forwardRef<HTMLInputElement, PhoneInputProps>(
  function PhoneInput(props, ref) {
    return (
      <Input
        ref={ref}
        type="tel"
        autoComplete="tel"
        placeholder="+7 999 123-45-67"
        {...props}
      />
    );
  },
);
