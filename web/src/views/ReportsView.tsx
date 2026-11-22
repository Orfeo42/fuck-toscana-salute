import {
  Alert,
  Badge,
  Button,
  Group,
  Loader,
  Text,
  TextInput,
  Title,
  Tooltip,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { Link2, RefreshCw, Search, Upload } from "lucide-react";
import { DataTable, type DataTableColumn } from "mantine-datatable";
import { useState } from "react";
import type { JSX, KeyboardEvent } from "react";

import type { ReportRowBody, ReportsQueryRequest } from "../api/client";
import { startRefresh } from "../data/refresh";
import { formatDate } from "../lib/format";
import { useSort } from "../lib/useSort";
import { useViewQuery } from "../lib/useViewQuery";
import { StatusKeyFilter } from "../components/StatusKeyFilter";
import { tableScrollProps } from "../components/tableScroll";
import { tableColors } from "../theme";
import { ReportDetailModal } from "./reports/ReportDetailModal";
import { UploadReportDialog } from "./reports/UploadReportDialog";

type ReportKind = NonNullable<ReportsQueryRequest["excludeKinds"]>[number];
type ReportSort = ReportsQueryRequest["sort"];
type PageSize = ReportsQueryRequest["pageSize"];

const reportKinds: readonly ReportKind[] = [
  "lab",
  "labtrasf",
  "ris",
  "ldo",
  "ps",
  "rsa",
  "rap",
  "local",
];
const sortKeys: readonly ReportSort[] = ["date", "kind", "provider"];
const pageSizes: readonly PageSize[] = [25, 50, 100];
const descFirst: readonly ReportSort[] = ["date"];
const rowPointer = { cursor: "pointer" };
const fseHint =
  "Accedi al Fascicolo Sanitario per vedere anche i referti del portale regionale.";

function sortKeyOf(accessor: string): ReportSort | undefined {
  const name = accessor === "kindLabel" ? "kind" : accessor;
  return sortKeys.find((key) => key === name);
}

export function ReportsView(): JSX.Element {
  const [search, setSearch] = useState("");
  const [excludeKinds, setExcludeKinds] = useState<ReportKind[] | undefined>(
    undefined,
  );
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState<PageSize>(50);
  const [starting, setStarting] = useState(false);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [opened, setOpened] = useState<ReportRowBody | undefined>(undefined);
  const sort = useSort<ReportSort>("date", "desc", descFirst);

  const request: ReportsQueryRequest = {
    search,
    sort: sort.key,
    dir: sort.dir,
    page,
    pageSize,
  };
  if (excludeKinds !== undefined) {
    request.excludeKinds = excludeKinds;
  }
  const reports = useViewQuery("/api/reports/query", request);
  const data = reports.data;

  const columns: DataTableColumn<ReportRowBody>[] = [
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
      filtering: (data?.excludedKinds.length ?? 0) > 0,
      filter: (
        <StatusKeyFilter
          options={data?.kinds ?? []}
          onChange={(keys) => {
            setExcludeKinds(reportKinds.filter((kind) => keys.includes(kind)));
            setPage(1);
          }}
        />
      ),
      render: (report) =>
        report.origin === "local" ? (
          <Group gap={6} wrap="nowrap">
            <Tooltip label="Caricato da te" withArrow>
              <span role="img" aria-label="Caricato da te">
                <Upload size={16} aria-hidden="true" />
              </span>
            </Tooltip>
            <Text size="sm">{report.title}</Text>
          </Group>
        ) : (
          report.kindLabel
        ),
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
          {report.linkedLabel === "" ? null : (
            <Tooltip label={report.linkedLabel} withArrow>
              <span role="img" aria-label={report.linkedLabel}>
                <Link2 size={16} aria-hidden="true" />
              </span>
            </Tooltip>
          )}
        </Group>
      ),
    },
  ];

  const refreshing = starting || data?.refreshing === true;
  const issue = reports.error === undefined ? data?.issue : undefined;

  const refreshAll = async (): Promise<void> => {
    setStarting(true);
    const ok = await startRefresh({ reports: true }).catch(() => false);
    setStarting(false);
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
            setPage(1);
          }}
        />
        {data === undefined ? null : (
          <Text aria-live="polite">{`${String(data.total)} referti`}</Text>
        )}
        {data === undefined || data.updated.label === "" ? null : (
          <Text size="sm" c="dimmed">
            {data.updated.label}
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
        <Button
          leftSection={<Upload size={16} aria-hidden="true" />}
          onClick={() => {
            setUploadOpen(true);
          }}
        >
          Carica referto
        </Button>
      </Group>
      {reports.loading && data === undefined ? (
        <Group gap="xs">
          <Loader size="sm" />
          <Text>Carico i referti…</Text>
        </Group>
      ) : null}
      {reports.error === "unauthorized" || issue === "login_required" ? (
        <Alert color="yellow" role="alert" mb="md">
          Accedi per vedere i referti.
        </Alert>
      ) : null}
      {(reports.error !== undefined && reports.error !== "unauthorized") ||
      issue === "unavailable" ? (
        <Alert color="red" role="alert" mb="md">
          Il portale regionale non risponde. Riprova tra poco.
        </Alert>
      ) : null}
      {data?.fseUnavailable === true ? (
        <Alert color="yellow" role="alert" mb="md" p="xs">
          {fseHint}
        </Alert>
      ) : null}
      {data !== undefined && data.failures.length > 0 ? (
        <Alert color="yellow" role="alert" mb="md" p="xs">
          Alcune sezioni non sono state caricate:{" "}
          {data.failures.map((failure) => failure.kindLabel).join(", ")}
        </Alert>
      ) : null}
      {data === undefined ? null : (
        <DataTable
          striped
          highlightOnHover
          withTableBorder
          {...tableColors}
          {...tableScrollProps}
          idAccessor="id"
          records={data.rows}
          columns={columns}
          fetching={reports.loading}
          noRecordsText="Nessun referto disponibile"
          totalRecords={data.total}
          page={page}
          onPageChange={setPage}
          recordsPerPage={pageSize}
          recordsPerPageLabel="Righe per pagina"
          recordsPerPageOptions={[...pageSizes]}
          onRecordsPerPageChange={(size) => {
            setPageSize(pageSizes.find((item) => item === size) ?? 50);
            setPage(1);
          }}
          sortStatus={{
            columnAccessor: sort.key === "kind" ? "kindLabel" : sort.key,
            direction: sort.dir,
          }}
          onSortStatusChange={(status) => {
            const key = sortKeyOf(String(status.columnAccessor));
            if (key !== undefined) {
              sort.toggle(key);
              setPage(1);
            }
          }}
          onRowClick={({ record }) => {
            setOpened(record);
          }}
          customRowAttributes={(report) => ({
            tabIndex: 0,
            onKeyDown: (event: KeyboardEvent<HTMLElement>) => {
              if (
                event.key === "Enter" &&
                event.target === event.currentTarget
              ) {
                setOpened(report);
              }
            },
          })}
          rowStyle={() => rowPointer}
        />
      )}
      {opened === undefined ? null : (
        <ReportDetailModal
          key={opened.id}
          reportRef={opened.ref}
          onClose={() => {
            setOpened(undefined);
          }}
          onChanged={reports.reload}
        />
      )}
      {uploadOpen ? (
        <UploadReportDialog
          opened
          onClose={() => {
            setUploadOpen(false);
          }}
          onUploaded={reports.reload}
        />
      ) : null}
    </section>
  );
}
