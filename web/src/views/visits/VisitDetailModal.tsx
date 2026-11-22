import { Paper, Stack, Text } from "@mantine/core";
import { MapPinned, Navigation, RefreshCw } from "lucide-react";
import type { JSX } from "react";

import type { AppointmentBody, ReportBody, RowBody } from "../../api/client";
import { CopyButton } from "../../components/CopyButton";
import { PdfLink } from "../../components/PdfLink";
import { DetailDrawer, type DetailTab } from "../../components/DetailDrawer";
import { DetailField, DetailSection } from "../../components/DetailSection";
import { IconAction } from "../../components/IconAction";
import { StatusBadge } from "../../components/StatusBadge";
import {
  formatDate,
  formatLongDateTime,
  formatShortDateTime,
} from "../../lib/format";
import { rowDate, rowTitle, unitName, type EffectiveState } from "./filter";
import { fullAddress, mapsDirectionsUrl, mapsSearchUrl } from "./mapsUrl";
import { BookAction } from "./BookAction";
import { CalendarAction } from "./CalendarAction";
import { PromemoriaLink } from "./PromemoriaLink";
import { relativeDay } from "./relativeDay";
import { linkedReports } from "./reportLinks";
import { statusDetails } from "./statusDetails";
import { VisitReportsSection } from "./VisitReportsSection";
import { VisitStatusSection } from "./VisitStatusSection";

interface Props {
  row: RowBody;
  state: EffectiveState;
  booking: AppointmentBody | undefined;
  now: Date;
  appointmentCheckedAt: string | undefined;
  refreshing: boolean;
  reports: readonly ReportBody[];
  onChanged: () => void;
  onRefresh: () => void;
  onClose: () => void;
}

export function VisitDetailModal({
  row,
  state,
  booking,
  now,
  appointmentCheckedAt,
  refreshing,
  reports,
  onChanged,
  onRefresh,
  onClose,
}: Props): JSX.Element {
  const details = statusDetails(row, state);
  const bookedUnit = state.kind === "booked" ? booking?.unit : undefined;
  const place = bookedUnit?.place ?? "";
  const address = bookedUnit === undefined ? "" : fullAddress(bookedUnit);
  const at = state.kind === "booked" ? state.at : undefined;
  const bookingId = state.kind === "booked" ? booking?.bookingId : undefined;
  const calendarBooking =
    booking !== undefined &&
    at !== undefined &&
    new Date(at).getTime() > now.getTime()
      ? booking
      : undefined;
  const verifiedAt = state.kind === "booked" ? undefined : state.verifiedAt;
  const updates = [
    appointmentCheckedAt === undefined
      ? undefined
      : `Appuntamenti: ${formatShortDateTime(appointmentCheckedAt)}`,
    verifiedAt === undefined
      ? undefined
      : `Verifica CUP: ${formatShortDateTime(verifiedAt)}`,
  ].filter((item) => item !== undefined);

  const tabs: DetailTab[] = [
    {
      value: "panoramica",
      label: "Panoramica",
      content: (
        <>
          <Paper withBorder p="sm" radius="md">
            <DetailSection
              title="Stato"
              actions={
                state.kind === "official" && state.canBook ? (
                  <BookAction />
                ) : undefined
              }
            >
              <Stack gap={4}>
                {details.map((detail) => (
                  <Text key={detail} size="sm">
                    {detail}
                  </Text>
                ))}
              </Stack>
              <VisitStatusSection row={row} onChanged={onChanged} />
            </DetailSection>
          </Paper>
          {bookedUnit === undefined ? null : (
            <Paper withBorder p="sm" radius="md">
              <DetailSection
                title="Appuntamento"
                actions={
                  <>
                    <PromemoriaLink bookingId={bookingId} />
                    {calendarBooking === undefined ? null : (
                      <CalendarAction
                        appointment={calendarBooking}
                        title={rowTitle(row)}
                      />
                    )}
                    {bookedUnit.address === "" ? null : (
                      <>
                        <IconAction
                          label="Indicazioni stradali"
                          icon={Navigation}
                          href={mapsDirectionsUrl(bookedUnit)}
                          external
                        />
                        <IconAction
                          label="Apri in Google Maps"
                          icon={MapPinned}
                          href={mapsSearchUrl(bookedUnit)}
                          external
                        />
                      </>
                    )}
                  </>
                }
              >
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
              </DetailSection>
            </Paper>
          )}
        </>
      ),
    },
    {
      value: "referti",
      label: "Referti",
      count: linkedReports(row, reports).length,
      content: (
        <VisitReportsSection
          row={row}
          state={state}
          booking={booking}
          now={now}
          reports={reports}
          onChanged={onChanged}
        />
      ),
    },
    {
      value: "prescrizione",
      label: "Prescrizione",
      content: (
        <Paper withBorder p="sm" radius="md">
          <DetailSection
            title="Prescrizione"
            actions={<PdfLink documentRef={row.fse?.documentRef} />}
          >
            <DetailField label="Data prescrizione">
              {formatDate(rowDate(row))}
            </DetailField>
            <DetailField label="NRE">
              <CopyButton value={row.nre} label="Copia NRE" />
            </DetailField>
          </DetailSection>
        </Paper>
      ),
    },
  ];

  return (
    <DetailDrawer
      title={rowTitle(row)}
      badge={
        <StatusBadge tone={state.tone} manual={state.manual}>
          {state.label}
        </StatusBadge>
      }
      summary={
        at === undefined || bookedUnit === undefined ? (
          <Text size="sm">{`Prescritta il ${formatDate(rowDate(row))}`}</Text>
        ) : (
          <>
            <Text size="sm">
              {`${formatLongDateTime(at)} · ${relativeDay(at, now)}`}
            </Text>
            <Text size="sm">{unitName(bookedUnit)}</Text>
          </>
        )
      }
      actions={
        <IconAction
          label="Aggiorna"
          icon={RefreshCw}
          loading={refreshing}
          onClick={onRefresh}
        />
      }
      tabs={tabs}
      footer={
        updates.length === 0
          ? undefined
          : `Aggiornamenti — ${updates.join(" · ")}`
      }
      onClose={onClose}
    />
  );
}
