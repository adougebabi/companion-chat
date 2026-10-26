import {
  Sheet,
  SheetContent,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import * as React from "react";
import { Slot } from "radix-ui";
import { ChevronDownIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea as ShadcnTextarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import {
  Collapsible,
  CollapsibleTrigger,
  CollapsibleContent,
} from "@/components/ui/collapsible";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
export { Label, Badge };
export function Textarea(props: React.ComponentProps<typeof ShadcnTextarea>) {
  return (
    <ShadcnTextarea
      {...props}
      {...(Object.hasOwn(props, "value") ? { value: props.value ?? "" } : {})}
    />
  );
}

export function ActionButton({
  className = "",
  variant,
  type,
  ...props
}: React.ComponentProps<typeof Button>) {
  const tokens = className.split(/\s+/);
  const selected =
    variant ??
    (tokens.includes("primary")
      ? "default"
      : (tokens.includes("danger") || tokens.includes("destructive"))
        ? "destructive"
        : tokens.includes("button")
          ? "outline"
          : "ghost");
  return (
    <Button
      className={cn("product-button", className)}
      variant={selected}
      type={type}
      {...props}
    />
  );
}
export function Surface({
  as: Tag = "div",
  className,
  children,
  ...props
}: React.HTMLAttributes<HTMLElement> & { as?: React.ElementType }) {
  if (/\bcard\b/.test(className || ""))
    return (
      <Card asChild>
        <Tag className={className} {...props}>
          {children}
        </Tag>
      </Card>
    );
  return (
    <Tag className={className} {...props}>
      {children}
    </Tag>
  );
}
type FieldProps = Omit<
  React.ComponentProps<typeof Input>,
  "value" | "onChange"
> & {
  value?: any;
  modelValue?: any;
  onModelChange?: (value: any) => void;
  onChange?: (event: any) => void;
};
export function FieldInput({
  type = "text",
  modelValue,
  onModelChange,
  onChange,
  onInput,
  value,
  checked,
  className,
  ...props
}: FieldProps) {
  const dom = React.useRef<HTMLButtonElement | null>(null);
  if (type === "checkbox") {
    const Component = /\bswitch\b/.test(className || "") ? Switch : Checkbox;
    const selected = onModelChange
      ? Array.isArray(modelValue)
        ? modelValue.includes(value)
        : Boolean(modelValue)
      : Boolean(checked);
    return (
      <span className="relative inline-flex shrink-0 align-middle">
      <Component
        {...(props as any)}
        ref={dom}
        className={className}
        checked={selected}
        onCheckedChange={(next) => {
          const checked = next === true;
          onModelChange?.(
            Array.isArray(modelValue)
              ? checked
                ? [...modelValue, value]
                : modelValue.filter((item: any) => item !== value)
              : checked,
          );
          const event = {
            target: { ...dom.current, value, checked },
            currentTarget: dom.current,
          };
          onChange?.(event);
          (onInput as any)?.(event);
        }}
      />
      </span>
    );
  }
  return (
    <Input
      type={type}
      className={className}
      value={type === "file" ? undefined : (value ?? "")}
      onChange={onChange}
      onInput={onInput}
      {...props}
    />
  );
}
function flatten(nodes: React.ReactNode): React.ReactElement<any>[] {
  return React.Children.toArray(nodes).flatMap((node) => {
    if (!React.isValidElement(node)) return [];
    const child = node as React.ReactElement<any>;
    return child.type === React.Fragment
      ? flatten(child.props.children)
      : [child];
  });
}
const optionValue = (option: React.ReactElement<any>) =>
  Object.hasOwn(option.props, "value")
    ? option.props.value
    : option.props.children;
/** Preserve exact domain values, including empty strings, booleans and numbers. */
export function ChoiceSelect({
  children,
  value,
  onChange,
  onInput,
  disabled,
  className,
  id,
  name,
  required,
  ...props
}: any) {
  const options = flatten(children).filter((item) => item.type === "option");
  const selected = options.findIndex((option) =>
    Object.is(optionValue(option), value),
  );
  const trigger = React.useRef<HTMLButtonElement | null>(null);
  return (
    <span className="relative inline-flex w-full min-w-0">
    <Select
      value={selected < 0 ? "" : `choice:${selected}`}
      disabled={disabled}
      name={name}
      required={required}
      onValueChange={(key) => {
        const option = options[Number(key.slice(7))];
        if (!option) return;
        const event = {
          target: { value: optionValue(option) },
          currentTarget: trigger.current,
        };
        onChange?.(event);
        onInput?.(event);
      }}
    >
      <SelectTrigger
        {...props}
        id={id}
        ref={trigger}
        className={cn("w-full min-w-0", className)}
      >
        <SelectValue placeholder="请选择">
          {selected >= 0 ? options[selected].props.children : undefined}
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        {options.map((option, index) => (
          <SelectItem
            key={String(option.key ?? index)}
            value={`choice:${index}`}
            disabled={option.props.disabled}
          >
            {option.props.children}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
    </span>
  );
}
export function Disclosure({
  title,
  children,
  open: externalOpen,
  className,
  id,
  ...props
}: any) {
  const [open, setOpen] = React.useState(Boolean(externalOpen));
  React.useEffect(() => {
    if (externalOpen !== undefined) setOpen(Boolean(externalOpen));
  }, [externalOpen]);
  return (
    <Collapsible
      {...props}
      id={id}
      open={open}
      onOpenChange={setOpen}
      className={cn("product-disclosure", className)}
    >
      <CollapsibleTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          className="disclosure-summary w-full justify-start whitespace-normal text-left"
        >
          <span className="min-w-0 flex-1">{title}</span>
          <ChevronDownIcon
            className={cn(
              "size-4 shrink-0 transition-transform",
              open && "rotate-180",
            )}
          />
        </Button>
      </CollapsibleTrigger>
      <CollapsibleContent className="disclosure-body">
        {children}
      </CollapsibleContent>
    </Collapsible>
  );
}
export function MenuDisclosure({ title, children, className, ...props }: any) {
  const actions = flatten(children).flatMap((child) =>
    child.type === "div" ? flatten(child.props.children) : [child],
  );
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label={props["aria-label"] || "更多操作"}
        >
          {title}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {actions.map((action, index) => (
          <DropdownMenuItem key={action.key ?? index} asChild>
            {action}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
function headingText(children: React.ReactNode): React.ReactNode {
  for (const node of flatten(children)) {
    if (["h1", "h2", "h3"].includes(node.type as string))
      return node.props.children;
    const nested = headingText(node.props.children);
    if (nested) return nested;
  }
  return null;
}
export function DialogFrame({ children, onClose, className, ...props }: any) {
  const [trigger] = React.useState(
    () => document.activeElement as HTMLElement | null,
  );
  const title = headingText(children) || props["aria-label"] || "详情";
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose?.();
      }}
    >
      <DialogContent
        className="react-workspace product-dialog max-h-[90dvh] overflow-y-auto"
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          (trigger?.isConnected
            ? trigger
            : document.querySelector<HTMLElement>("main")
          )?.focus({ preventScroll: true });
        }}
      >
        <DialogTitle className="sr-only">{title}</DialogTitle>
        <DialogDescription className="sr-only">
          查看并处理当前内容
        </DialogDescription>
        {children}
      </DialogContent>
    </Dialog>
  );
}

export function DrawerFrame({ children, onClose }: any) {
  const [trigger] = React.useState(
    () => document.activeElement as HTMLElement | null,
  );
  return (
    <Sheet
      open
      onOpenChange={(open) => {
        if (!open) onClose?.();
      }}
    >
      <SheetContent
        className="react-workspace product-sheet overflow-y-auto"
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          (trigger?.isConnected
            ? trigger
            : document.querySelector<HTMLElement>("main")
          )?.focus({ preventScroll: true });
        }}
      >
        <SheetTitle className="sr-only">
          {headingText(children) || "编辑详情"}
        </SheetTitle>
        <SheetDescription className="sr-only">
          编辑当前对象，取消时保留未提交内容
        </SheetDescription>
        <div className="p-6">{children}</div>
      </SheetContent>
    </Sheet>
  );
}
