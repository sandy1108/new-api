import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

// 账本筛选器只组合现有Select，不再实现弹层、键盘或焦点行为。
export function QuotaSelect(props: {
  label: string
  value: string
  onChange: (value: string) => void
  options: { value: string; label: string }[]
  disabled?: boolean
}) {
  return (
    <Select
      value={
        props.options.some((item) => item.value === props.value)
          ? props.value
          : null
      }
      onValueChange={(value) => {
        if (value !== null) props.onChange(value)
      }}
      items={props.options}
      disabled={props.disabled}
    >
      <SelectTrigger aria-label={props.label} className='max-w-full min-w-36'>
        <SelectValue placeholder={props.label} />
      </SelectTrigger>
      <SelectContent>
        {props.options.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
