import { ActionIcon, Tooltip } from "@mantine/core";
import type { LucideIcon } from "lucide-react";
import type { JSX } from "react";

export type IconActionTone = "default" | "primary" | "danger";

interface BaseProps {
  label: string;
  icon: LucideIcon;
  tone?: IconActionTone;
  disabled?: boolean;
}

interface LinkProps extends BaseProps {
  href: string;
  external?: boolean;
  onClick?: undefined;
  type?: undefined;
  loading?: undefined;
}

interface ButtonProps extends BaseProps {
  onClick: () => void;
  type?: "button";
  loading?: boolean;
  href?: undefined;
  external?: undefined;
}

interface SubmitProps extends BaseProps {
  type: "submit";
  loading?: boolean;
  onClick?: undefined;
  href?: undefined;
  external?: undefined;
}

type Props = LinkProps | ButtonProps | SubmitProps;

const toneStyle: Record<IconActionTone, { variant: string; color?: string }> = {
  default: { variant: "subtle" },
  primary: { variant: "filled" },
  danger: { variant: "light", color: "red" },
};

export function IconAction(props: Props): JSX.Element {
  const { label, icon: Icon, tone = "default", disabled = false } = props;
  const style = toneStyle[tone];
  const icon = <Icon aria-hidden="true" size={18} />;

  if (props.href !== undefined) {
    return (
      <Tooltip label={label} withArrow openDelay={150}>
        <ActionIcon
          component="a"
          {...style}
          size="lg"
          aria-label={label}
          href={props.href}
          target={props.external === true ? "_blank" : undefined}
          rel={props.external === true ? "noopener noreferrer" : undefined}
        >
          {icon}
        </ActionIcon>
      </Tooltip>
    );
  }

  return (
    <Tooltip label={label} withArrow openDelay={150}>
      <ActionIcon
        type={props.type ?? "button"}
        {...style}
        size="lg"
        aria-label={label}
        loading={props.loading ?? false}
        disabled={disabled}
        onClick={() => {
          props.onClick?.();
        }}
      >
        {icon}
      </ActionIcon>
    </Tooltip>
  );
}
