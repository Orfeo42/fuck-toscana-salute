import { ActionIcon, Tooltip } from "@mantine/core";
import type { LucideIcon } from "lucide-react";
import type { JSX } from "react";

interface BaseProps {
  label: string;
  icon: LucideIcon;
}

interface LinkProps extends BaseProps {
  href: string;
  external?: boolean;
  onClick?: undefined;
}

interface ButtonProps extends BaseProps {
  onClick: () => void;
  href?: undefined;
  external?: undefined;
}

type Props = LinkProps | ButtonProps;

export function IconAction(props: Props): JSX.Element {
  const { label, icon: Icon } = props;
  const icon = <Icon aria-hidden="true" size={18} />;

  if (props.href === undefined) {
    return (
      <Tooltip label={label} withArrow openDelay={150}>
        <ActionIcon
          variant="subtle"
          size="lg"
          aria-label={label}
          onClick={props.onClick}
        >
          {icon}
        </ActionIcon>
      </Tooltip>
    );
  }

  return (
    <Tooltip label={label} withArrow openDelay={150}>
      <ActionIcon
        component="a"
        variant="subtle"
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
