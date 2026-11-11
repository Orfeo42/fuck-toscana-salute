import { Modal, Stack } from "@mantine/core";
import { useMediaQuery } from "@mantine/hooks";
import type { JSX } from "react";

import { CopyButton } from "../../components/CopyButton";
import { DetailField, DetailSection } from "../../components/DetailSection";
import { PdfLink } from "../../components/PdfLink";
import { StatusBadge } from "../../components/StatusBadge";
import { formatDate } from "../../lib/format";
import { drugStatusLabel, drugStatusTone, type DrugRowData } from "./filter";

interface Props {
  row: DrugRowData;
  onClose: () => void;
}

export function DrugDetailModal({ row, onClose }: Props): JSX.Element {
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
          <PdfLink documentRef={fse.documentRef} labelled />
        </DetailSection>
      </Stack>
    </Modal>
  );
}
