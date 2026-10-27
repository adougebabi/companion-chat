import FilePicker from './shared/FilePicker';
import {InputGroup,InputGroupAddon} from '@/components/ui/input-group';
import {ClockIcon} from 'lucide-react';
import {MinusIcon,PlusIcon} from 'lucide-react';
import ControlHint from './shared/ControlHint';
import DateField from './shared/DateField';
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
import { Label as PrimitiveLabel } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { Switch } from "@/components/ui/switch";
import { Badge as ShadcnBadge } from "@/components/ui/badge";
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
export function Label({children,className,...props}:React.ComponentProps<typeof PrimitiveLabel>){
 const checkable=React.Children.toArray(children).some(child=>React.isValidElement(child)&&!!child.props&&typeof child.props==='object'&&'type' in child.props&&(child.props.type==='checkbox'||child.props.type==='radio'));
 return <PrimitiveLabel {...props} className={cn('min-w-0',checkable?'flex-row items-center':'flex-col items-start',className)}>{children}</PrimitiveLabel>;
}
export function Badge({className='',title,variant,...props}:React.ComponentProps<typeof ShadcnBadge>){return <ControlHint text={title}><ShadcnBadge variant={variant||'secondary'} className={className} {...props}/></ControlHint>;}


// React owns the immediate edit value; domain controllers may notify later.
// Keep the same native element and never restore stale props during composition.
function DraftTextControl({ multiline = false, value, onChange, onCompositionStart, onCompositionEnd, ...props }: any) {
  const external = String(value ?? "");
  const [draft, setDraft] = React.useState(external);
  const composing = React.useRef(false);
  const observed = React.useRef(external);
  React.useLayoutEffect(() => {
    if (observed.current === external) return;
    observed.current = external;
    if (!composing.current) setDraft(external);
  }, [external]);
  const Component = multiline ? ShadcnTextarea : Input;
  return <Component {...props} value={draft}
    onChange={(event: any) => { setDraft(event.target.value); onChange?.(event); }}
    onCompositionStart={(event: any) => { composing.current = true; onCompositionStart?.(event); }}
    onCompositionEnd={(event: any) => { composing.current = false; setDraft(event.currentTarget.value); onCompositionEnd?.(event); }} />;
}
export function Textarea(props: React.ComponentProps<typeof ShadcnTextarea>) {
  return Object.hasOwn(props, 'value') ? <DraftTextControl {...props} multiline /> : <ShadcnTextarea {...props}/>;
}

export function ActionButton({
  className = "",
  variant,
  type,
  title,
  children,
  ...props
}: React.ComponentProps<typeof Button>) {

  const intrinsic=!className&&props['aria-expanded']===undefined&&props.role!=='menuitem';
  return <ControlHint text={title}><Button className={cn(intrinsic&&'w-fit max-w-full',className)} variant={variant??'ghost'} type={type} {...props}>{children}</Button></ControlHint>;
}

export function Surface({
  as: Tag = "div",
  className,
  children,
  ...props
}: React.HTMLAttributes<HTMLElement> & { as?: React.ElementType }) {
  return <Card asChild className={cn('min-w-0 gap-4 p-5',className?.includes('media-tile')&&'p-0',className)}><Tag {...props}>{children}</Tag></Card>;
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
function FieldControl({
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
  if (type === "date") return <DateField value={value} onChange={onChange} onInput={onInput} {...props}/>;
  if (type === "time") return <InputGroup className="h-auto min-h-11"><InputGroupAddon><ClockIcon/></InputGroupAddon><DraftTextControl {...props} type="text" inputMode="numeric" placeholder={props.placeholder||'HH:mm'} pattern={props.pattern||'(?:[01][0-9]|2[0-3]):[0-5][0-9](?::[0-5][0-9])?'} value={value} onChange={onChange} onInput={onInput} className={cn('border-0 shadow-none focus-visible:ring-0',className)} data-slot="input-group-control"/></InputGroup>;
  if (type === "file") return <FilePicker onChange={onChange} onInput={onInput} {...props}/>;
  if (type === "number") return <NumberControl className={className} value={value} onChange={onChange} onInput={onInput} {...props}/>;
  return <DraftTextControl type={type} className={className} value={value} onChange={onChange} onInput={onInput} {...props}/>;
}
function NumberControl(props:any){const input=React.useRef<HTMLInputElement>(null);const step=(direction:number)=>{const element=input.current;if(!element)return;try{if(direction>0)element.stepUp();else element.stepDown();}catch{const current=Number(element.value)||0,min=element.min===''?-Infinity:Number(element.min),max=element.max===''?Infinity:Number(element.max);Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value')!.set!.call(element,String(Math.max(min,Math.min(max,current+direction))));}element.dispatchEvent(new Event('input',{bubbles:true}));};return <span className="inline-flex w-full min-w-0 items-center gap-1"><Button type="button" variant="outline" size="icon" aria-label={'减少'+(props['aria-label']||props.name||'数值')} disabled={props.disabled||props.readOnly} onClick={()=>step(-1)}><MinusIcon className="size-3.5"/></Button><DraftTextControl {...props} ref={input} type="number" className={cn('min-w-0 [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none',props.className)}/><Button type="button" variant="outline" size="icon" aria-label={'增加'+(props['aria-label']||props.name||'数值')} disabled={props.disabled||props.readOnly} onClick={()=>step(1)}><PlusIcon className="size-3.5"/></Button></span>;}
export function FieldInput({title,...props}:FieldProps){return <ControlHint text={title}><FieldControl {...props}/></ControlHint>;}

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
  const options = flatten(children).filter((item) => item.type === "option").filter((item, index, all) => all.findIndex(candidate => Object.is(optionValue(candidate), optionValue(item))) === index);
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
      <DropdownMenuContent align="end" sideOffset={6} collisionPadding={8}>
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
