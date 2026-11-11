import {
  Alert,
  Badge,
  Button,
  Group,
  Loader,
  NativeSelect,
  Text,
  TextInput,
  Title,
  Tooltip,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { RefreshCw, Search } from "lucide-react";
import { DataTable, type DataTableColumn } from "mantine-datatable";
import { useState } from "react";
import type { JSX, KeyboardEvent } from "react";

import type { ReportBody } from "../api/client";
import { refreshPortals } from "../data/refresh";
import { formatDate } from "../lib/format";
import { useSort } from "../lib/useSort";
import { tableColors } from "../theme";
import {
  filterByKind,
  kindLabels,
  reportId,
  reportSortKeys,
  searchReports,
  sortReports,
  type ReportSortKey,
} from "./reports/filter";
import { ReportDetailModal } from "./reports/ReportDetailModal";
import { updatedLabel } from "./reports/updatedLabel";
import { useReports } from "./reports/useReports";

const descFirst: readonly ReportSortKey[] = ["date"];
const rowPointer = { cursor: "pointer" };
const allKinds = "";

const columns: DataTableColumn<ReportBody>[] = [
  {
    accessor: "date",
    title: "Data",
    sortable: true,
    noWrap: true,
    render: (report) => formatDate(report.date),
  },
  {
    accessor: "kindLabel",
    title: "Tipo",
    sortable: true,
    noWrap: true,
  },
  {
    accessor: "provider",
    title: "Struttura",
    sortable: true,
    noWrap: true,
    width: 320,
    render: (report) => (
      <Tooltip label={report.provider} openDelay={300}>
        <Text truncate="end" size="sm" maw={300}>
          {report.provider}
        </Text>
      </Tooltip>
    ),
  },
  {
    accessor: "unread",
    title: "Stato",
    noWrap: true,
    render: (report) => (
      <Group gap={4} wrap="nowrap">
        {report.unread ? (
          <Badge size="sm" color="blue">
            Nuovo
          </Badge>
        ) : null}
        {report.hidden ? (
          <Badge size="sm" color="gray" variant="light">
            Oscurato
          </Badge>
        ) : null}
      </Group>
    ),
  },
];

interface Props {
  now?: Date;
}

export function ReportsView({ now }: Props = {}): JSX.Element {
  const reports = useReports();
  const [mountedAt] = useState(() => new Date());
  const [search, setSearch] = useState("");
  const [kind, setKind] = useState(allKinds);
  const [refreshing, setRefreshing] = useState(false);
  const [detailId, setDetailId] = useState<string | undefined>(undefined);
  const sort = useSort<ReportSortKey>("date", "desc", descFirst);

  const all = reports.data?.reports ?? [];
  const failures = reports.data?.failures ?? [];
  const updated = reports.data?.updated ?? undefined;
  const visible = sortReports(
    filterByKind(searchReports(all, search), kind),
    sort.key,
    sort.dir,
  );
  const detail =
    detailId === undefined
      ? undefined
      : all.find((report) => reportId(report) === detailId);

  const refreshAll = async (): Promise<void> => {
    setRefreshing(true);
    const ok = await refreshPortals({ all: true }).catch(() => false);
    setRefreshing(false);
    if (!ok) {
      notifications.show({
        color: "red",
        message: "Aggiornamento non riuscito",
      });
    }
    reports.reload();
  };

  return (
    <section aria-labelledby="reports-title" aria-busy={reports.loading}>
      <Title order={2} id="reports-title">
        Referti
      </Title>
      <Group align="flex-end" wrap="wrap" mb="md">
        <TextInput
          label="Cerca"
          type="search"
          leftSection={<Search size={16} aria-hidden="true" />}
          value={search}
          onChange={(event) => {
            setSearch(event.currentTarget.value);
          }}
        />
        <NativeSelect
          label="Tipo"
          value={kind}
          data={[
            { value: allKinds, label: "Tutti i tipi" },
            ...kindLabels(all).map((label) => ({ value: label, label })),
          ]}
          onChange={(event) => {
            setKind(event.currentTarget.value);
          }}
        />
        <Text aria-live="polite">{`${String(visible.length)} referti`}</Text>
        {updated === undefined ? null : (
          <Text size="sm" c="dimmed">
            {updatedLabel(updated, now ?? mountedAt)}
          </Text>
        )}
        <Button
          variant="light"
          leftSection={<RefreshCw size={16} aria-hidden="true" />}
          loading={refreshing}
          onClick={() => {
            void refreshAll();
          }}
        >
          Aggiorna tutto
        </Button>
      </Group>
      {reports.loading ? (
        <Group gap="xs">
          <Loader size="sm" />
          <Text>Carico i referti…</Text>
        </Group>
      ) : null}
      {!reports.loading && reports.error === "unauthorized" ? (
        <Alert color="yellow" role="alert" mb="md">
          Accedi per vedere i referti.
        </Alert>
      ) : null}
      {!reports.loading && reports.error === "unavailable" ? (
        <Alert color="red" role="alert" mb="md">
          Il portale regionale non risponde. Riprova tra poco.
        </Alert>
      ) : null}
      {failures.length > 0 ? (
        <Alert color="yellow" role="alert" mb="md" p="xs">
          Alcune sezioni non sono state caricate:{" "}
          {failures.map((failure) => failure.kindLabel).join(", ")}
        </Alert>
      ) : null}
      {reports.data === undefined ? null : (
        <DataTable
          striped
          highlightOnHover
          withTableBorder
          {...tableColors}
          maxHeight="75vh"
          idAccessor={reportId}
          records={visible}
          columns={columns}
          noRecordsText="Nessun referto disponibile"
          sortStatus={{ columnAccessor: sort.key, direction: sort.dir }}
          onSortStatusChange={(status) => {
            const key = reportSortKeys.find(
              (item) => item === status.columnAccessor,
            );
            if (key !== undefined) {
              sort.toggle(key);
            }
          }}
          onRowClick={({ record }) => {
            setDetailId(reportId(record));
          }}
          customRowAttributes={(report) => ({
            tabIndex: 0,
            onKeyDown: (event: KeyboardEvent<HTMLElement>) => {
              if (
                event.key === "Enter" &&
                event.target === event.currentTarget
              ) {
                setDetailId(reportId(report));
              }
            },
          })}
          rowStyle={() => rowPointer}
        />
      )}
      {detail === undefined ? null : (
        <ReportDetailModal
          key={reportId(detail)}
          report={detail}
          onClose={() => {
            setDetailId(undefined);
          }}
        />
      )}
    </section>
  );
}
