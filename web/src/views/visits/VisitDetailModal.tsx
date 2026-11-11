import { Button, Group, Modal, Stack, Text } from "@mantine/core";
import { useMediaQuery } from "@mantine/hooks";
import { MapPinned, Navigation } from "lucide-react";
import type { JSX } from "react";

import type { AppointmentBody, RowBody } from "../../api/client";
import { CopyButton } from "../../components/CopyButton";
import { DetailField, DetailSection } from "../../components/DetailSection";
import { StatusBadge } from "../../components/StatusBadge";
import { formatDate, formatLongDateTime } from "../../lib/format";
import { rowDate, rowTitle, unitName, type EffectiveState } from "./filter";
import { fullAddress, mapsDirectionsUrl, mapsSearchUrl } from "./mapsUrl";
import { relativeDay } from "./relativeDay";
import { statusDetails } from "./statusDetails";
import { ActionsCell } from "./VisitCells";

interface Props {
  row: RowBody;
  state: EffectiveState;
  booking: AppointmentBody | undefined;
  now: Date;
  onClose: () => void;
}

export function VisitDetailModal({
  row,
  state,
  booking,
  now,
  onClose,
}: Props): JSX.Element {
  const fullScreen = useMediaQuery("(max-width: 48em)");
  const details = statusDetails(row, state);
  const bookedUnit = state.kind === "booked" ? booking?.unit : undefined;
  const place = bookedUnit?.place ?? "";
  const address = bookedUnit === undefined ? "" : fullAddress(bookedUnit);
  const at = state.kind === "booked" ? state.at : undefined;

  return (
    <Modal
      opened
      onClose={onClose}
      title={rowTitle(row)}
      fullScreen={fullScreen}
      centered
    >
      <Stack gap="lg">
        <DetailSection title="Stato">
          <Group gap="sm" align="flex-start" wrap="nowrap">
            <StatusBadge tone={state.tone}>{state.label}</StatusBadge>
            <Stack gap={4}>
              {details.map((detail) => (
                <Text key={detail} size="sm">
                  {detail}
                </Text>
              ))}
            </Stack>
          </Group>
        </DetailSection>
        {bookedUnit === undefined ? null : (
          <DetailSection title="Appuntamento">
            {at === undefined ? null : (
              <DetailField label="Data e ora">
                <>
                  <Text>{formatLongDateTime(at)}</Text>
                  <Text size="sm" c="dimmed">
                    {relativeDay(at, now)}
                  </Text>
                </>
              </DetailField>
            )}
            <DetailField label="Struttura">
              <>
                <Text>{unitName(bookedUnit)}</Text>
                {bookedUnit.description !== "" &&
                bookedUnit.description !== bookedUnit.name ? (
                  <Text size="sm" c="dimmed">
                    {bookedUnit.name}
                  </Text>
                ) : null}
                {place === "" ? null : <Text size="sm">{place}</Text>}
                {address === "" ? null : <Text size="sm">{address}</Text>}
              </>
            </DetailField>
            {bookedUnit.address === "" ? null : (
              <Group gap="sm">
                <Button
                  component="a"
                  href={mapsDirectionsUrl(bookedUnit)}
                  target="_blank"
                  rel="noopener noreferrer"
                  leftSection={<Navigation size={16} aria-hidden="true" />}
                >
                  Indicazioni stradali
                </Button>
                <Button
                  component="a"
                  variant="light"
                  href={mapsSearchUrl(bookedUnit)}
                  target="_blank"
                  rel="noopener noreferrer"
                  leftSection={<MapPinned size={16} aria-hidden="true" />}
                >
                  Apri in Google Maps
                </Button>
              </Group>
            )}
          </DetailSection>
        )}
        <DetailSection title="Prescrizione">
          <DetailField label="Data prescrizione">
            {formatDate(rowDate(row))}
          </DetailField>
          <DetailField label="NRE">
            <CopyButton value={row.nre} label="Copia NRE" />
          </DetailField>
        </DetailSection>
        <DetailSection title="Azioni">
          <ActionsCell
            row={row}
            state={state}
            booking={booking}
            now={now}
            labelled
          />
        </DetailSection>
      </Stack>
    </Modal>
  );
}
