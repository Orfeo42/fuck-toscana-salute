import { ActionIcon, Button, Tooltip } from "@mantine/core";
import type { LucideIcon } from "lucide-react";
import type { JSX } from "react";

interface BaseProps {
  label: string;
  icon: LucideIcon;
  labelled?: boolean;
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

  if (props.href === undefined && props.labelled === true) {
    return (
      <Button variant="light" leftSection={icon} onClick={props.onClick}>
        {label}
      </Button>
    );
  }

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

  if (props.labelled === true) {
    return (
      <Button
        component="a"
        variant="light"
        leftSection={icon}
        href={props.href}
        target={props.external === true ? "_blank" : undefined}
        rel={props.external === true ? "noopener noreferrer" : undefined}
      >
        {label}
      </Button>
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
