import { Modal } from "@mantine/core";
import { useMediaQuery } from "@mantine/hooks";
import type { JSX } from "react";

interface Props {
  url: string;
  title: string;
  opened: boolean;
  onClose: () => void;
}

export function ImageViewerModal({
  url,
  title,
  opened,
  onClose,
}: Props): JSX.Element {
  const fullScreen = useMediaQuery("(max-width: 48em)");
  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title={title}
      size="xl"
      fullScreen={fullScreen}
      closeButtonProps={{ "aria-label": "Chiudi" }}
    >
      <img
        src={url}
        alt={title}
        style={{ maxWidth: "100%", display: "block" }}
      />
    </Modal>
  );
}
