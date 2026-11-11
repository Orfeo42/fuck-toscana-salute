import {
  Alert,
  Button,
  Group,
  Loader,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { RefreshCw, Search } from "lucide-react";
import { DataTable, type DataTableColumn } from "mantine-datatable";
import { useState } from "react";
import type { JSX, KeyboardEvent } from "react";

import { CopyButton } from "../components/CopyButton";
import { PdfLink } from "../components/PdfLink";
import { StopClick } from "../components/StopClick";
import { StatusBadge } from "../components/StatusBadge";
import { StatusColumnFilter } from "../components/StatusColumnFilter";
import { useHealthData } from "../data/useHealthData";
import { formatDate, formatUpdated } from "../lib/format";
import { tableColors } from "../theme";
import { focusId, useFocusTarget } from "../lib/useFocusTarget";
import { useSort } from "../lib/useSort";
import { DrugDetailModal } from "./drugs/DrugDetailModal";
import {
  defaultExcludedDrugLabels,
  drugLabelCounts,
  drugRows,
  drugSortKeys,
  drugStatusLabel,
  drugStatusTone,
  excludeDrugLabels,
  searchDrugs,
  sortDrugs,
  type DrugRowData,
  type DrugSortKey,
} from "./drugs/filter";

const descFirst: readonly DrugSortKey[] = ["data"];
const focusedBg = "var(--mantine-primary-color-light)";
const rowPointer = { cursor: "pointer" };

export function DrugsView(): JSX.Element {
  const health = useHealthData();
  const { refresh, refreshing } = health;
  const { data: body, error, loading } = health.prescriptions;
  const [now] = useState(() => new Date());
  const [selectedNres, setSelectedNres] = useState<readonly string[]>([]);
  const [handledFocus, setHandledFocus] = useState<string | undefined>(
    undefined,
  );
  const [excluded, setExcluded] = useState<readonly string[]>(
    defaultExcludedDrugLabels,
  );
  const [search, setSearch] = useState("");
  const [detailNre, setDetailNre] = useState<string | undefined>(undefined);
  const sort = useSort<DrugSortKey>("data", "desc", descFirst);
  const focus = useFocusTarget("farmaci", body !== undefined);

  const drugs = drugRows(body?.prescriptions ?? []);
  const searched = searchDrugs(drugs, search);
  const options = drugLabelCounts(searched);
  const visible = sortDrugs(
    excludeDrugLabels(searched, excluded),
    sort.key,
    sort.dir,
  );

  if (body !== undefined && focus !== handledFocus) {
    setHandledFocus(focus);
    const focused = drugs.find((row) => row.nre.toUpperCase() === focus);
    const shown = visible.some((row) => row.nre.toUpperCase() === focus);
    if (focused !== undefined && !shown) {
      const label = drugStatusLabel(focused.fse.status);
      setExcluded(excluded.filter((item) => item !== label));
      setSearch("");
    }
  }

  const selected = visible.filter((row) => selectedNres.includes(row.nre));
  const updatedText = formatUpdated(body?.updated, now);

  const columns: DataTableColumn<DrugRowData>[] = [
    {
      accessor: "data",
      title: "Data",
      sortable: true,
      noWrap: true,
      render: (row) => formatDate(row.fse.date),
    },
    {
      accessor: "farmaco",
      title: "Farmaco",
      sortable: true,
      render: (row) => row.fse.description || "Farmaco",
    },
    {
      accessor: "stato",
      title: "Stato",
      sortable: true,
      filtering: excluded.length > 0,
      filter: (
        <StatusColumnFilter
          options={options.map((option) => ({
            value: option.label,
            label: option.label,
            count: option.count,
          }))}
          selected={options
            .map((option) => option.label)
            .filter((label) => !excluded.includes(label))}
          onChange={(next) => {
            setExcluded([
              ...excluded.filter(
                (label) => !options.some((option) => option.label === label),
              ),
              ...options
                .map((option) => option.label)
                .filter((label) => !next.includes(label)),
            ]);
          }}
        />
      ),
      render: (row) => (
        <StatusBadge tone={drugStatusTone(row.fse.status)}>
          {drugStatusLabel(row.fse.status)}
        </StatusBadge>
      ),
    },
    {
      accessor: "nre",
      title: "NRE",
      sortable: true,
      noWrap: true,
      render: (row) => (
        <StopClick>
          <CopyButton value={row.nre} label="Copia NRE" />
        </StopClick>
      ),
    },
    {
      accessor: "azioni",
      title: "Azioni",
      render: (row) => (
        <StopClick>
          <Group gap={2} wrap="nowrap">
            <PdfLink documentRef={row.fse.documentRef} />
          </Group>
        </StopClick>
      ),
    },
  ];

  const detailRow =
    detailNre === undefined
      ? undefined
      : drugs.find((row) => row.nre === detailNre);
  const ready = !loading && error === undefined;

  return (
    <section aria-labelledby="drugs-title" aria-busy={loading}>
      <Title order={2} id="drugs-title">
        Farmaci
      </Title>
      <Group align="flex-end" wrap="wrap" mb="md">
        <TextInput
          label="Cerca"
          type="search"
          leftSection={<Search size={16} />}
          value={search}
          onChange={(event) => {
            setSearch(event.target.value);
          }}
        />
        <Text aria-live="polite">{`${String(visible.length)} farmaci`}</Text>
        {updatedText === undefined ? null : (
          <Text size="sm" c="dimmed">
            {updatedText}
          </Text>
        )}
        <Button
          variant="light"
          loading={refreshing}
          leftSection={<RefreshCw size={16} />}
          onClick={() => {
            void refresh({ all: true });
          }}
        >
          Aggiorna tutto
        </Button>
        {selected.length === 0 ? null : (
          <Button
            variant="light"
            loading={refreshing}
            leftSection={<RefreshCw size={16} />}
            onClick={() => {
              const nres = [
                ...new Set(
                  selected
                    .map((row) => row.nre.toUpperCase())
                    .filter((nre) => nre !== ""),
                ),
              ];
              setSelectedNres([]);
              void refresh({ nres });
            }}
          >
            {`Aggiorna selezionate (${String(selected.length)})`}
          </Button>
        )}
      </Group>
      {loading ? (
        <Group gap="xs">
          <Loader size="sm" />
          <Text>Carico i farmaci…</Text>
        </Group>
      ) : null}
      {!loading && error === "unauthorized" ? (
        <Alert color="yellow" role="alert">
          Accedi per vedere i farmaci.
        </Alert>
      ) : null}
      {!loading && error !== undefined && error !== "unauthorized" ? (
        <Alert color="red" role="alert">
          Il portale regionale non risponde. Riprova tra poco.
        </Alert>
      ) : null}
      {ready ? (
        <DataTable
          striped
          highlightOnHover
          withTableBorder
          {...tableColors}
          maxHeight="75vh"
          idAccessor="nre"
          records={visible}
          selectedRecords={selected}
          onSelectedRecordsChange={(records) => {
            setSelectedNres(records.map((row) => row.nre));
          }}
          columns={columns}
          noRecordsText="Nessun farmaco in questa vista."
          sortStatus={{ columnAccessor: sort.key, direction: sort.dir }}
          onSortStatusChange={(status) => {
            const key = drugSortKeys.find(
              (item) => item === status.columnAccessor,
            );
            if (key !== undefined) {
              sort.toggle(key);
            }
          }}
          onRowClick={({ record }) => {
            setDetailNre(record.nre);
          }}
          customRowAttributes={(row) => ({
            id: focusId("farmaci", row.nre),
            tabIndex: 0,
            onKeyDown: (event: KeyboardEvent<HTMLElement>) => {
              if (
                event.key === "Enter" &&
                event.target === event.currentTarget
              ) {
                setDetailNre(row.nre);
              }
            },
            "data-focused":
              row.nre.toUpperCase() === focus ? "true" : undefined,
          })}
          rowStyle={(row) =>
            row.nre.toUpperCase() === focus
              ? { ...rowPointer, backgroundColor: focusedBg }
              : rowPointer
          }
        />
      ) : null}
      {detailRow === undefined ? null : (
        <DrugDetailModal
          row={detailRow}
          refreshing={refreshing}
          onRefresh={() => {
            void refresh({ nres: [detailRow.nre.toUpperCase()] });
          }}
          onClose={() => {
            setDetailNre(undefined);
          }}
        />
      )}
    </section>
  );
}
