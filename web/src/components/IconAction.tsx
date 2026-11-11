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
  const content = (
    <>
      <Icon aria-hidden="true" size={18} />
      <span className="icon-tip" aria-hidden="true">
        {label}
      </span>
    </>
  );

  if (props.href === undefined) {
    return (
      <button
        type="button"
        className="icon-action"
        aria-label={label}
        onClick={props.onClick}
      >
        {content}
      </button>
    );
  }

  return (
    <a
      className="icon-action"
      aria-label={label}
      href={props.href}
      target={props.external === true ? "_blank" : undefined}
      rel={props.external === true ? "noopener noreferrer" : undefined}
    >
      {content}
    </a>
  );
}
