import { Group, Modal, Stack } from "@mantine/core";
import { useMediaQuery } from "@mantine/hooks";
import { RefreshCw } from "lucide-react";
import type { JSX } from "react";

import { CopyButton } from "../../components/CopyButton";
import { DetailField, DetailSection } from "../../components/DetailSection";
import { IconAction } from "../../components/IconAction";
import { PdfLink } from "../../components/PdfLink";
import { StatusBadge } from "../../components/StatusBadge";
import { formatDate } from "../../lib/format";
import { drugStatusLabel, drugStatusTone, type DrugRowData } from "./filter";

interface Props {
  row: DrugRowData;
  refreshing: boolean;
  onRefresh: () => void;
  onClose: () => void;
}

export function DrugDetailModal({
  row,
  refreshing,
  onRefresh,
  onClose,
}: Props): JSX.Element {
  const fullScreen = useMediaQuery("(max-width: 48em)");
  const { fse } = row;

  return (
    <Modal
      opened
      onClose={onClose}
      title={fse.description || "Farmaco"}
      fullScreen={fullScreen}
      centered
    >
      <Stack gap="lg">
        <DetailSection title="Stato">
          <StatusBadge tone={drugStatusTone(fse.status)}>
            {drugStatusLabel(fse.status)}
          </StatusBadge>
        </DetailSection>
        <DetailSection title="Prescrizione">
          <DetailField label="Data">{formatDate(fse.date)}</DetailField>
          <DetailField label="NRE">
            <CopyButton value={row.nre} label="Copia NRE" />
          </DetailField>
        </DetailSection>
        <DetailSection title="Azioni">
          <Group gap="xs">
            <PdfLink documentRef={fse.documentRef} />
            <IconAction
              label="Aggiorna"
              icon={RefreshCw}
              loading={refreshing}
              onClick={onRefresh}
            />
          </Group>
        </DetailSection>
      </Stack>
    </Modal>
  );
}
