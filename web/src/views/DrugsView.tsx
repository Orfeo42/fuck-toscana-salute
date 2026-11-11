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
import type { JSX } from "react";

import { CopyButton } from "../components/CopyButton";
import { PdfLink } from "../components/PdfLink";
import { StatusBadge } from "../components/StatusBadge";
import { StatusColumnFilter } from "../components/StatusColumnFilter";
import { useHealthData } from "../data/useHealthData";
import { formatDate } from "../lib/format";
import { focusId, useFocusTarget } from "../lib/useFocusTarget";
import { useSort } from "../lib/useSort";
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

export function DrugsView(): JSX.Element {
  const { data: body, error, loading, reload } = useHealthData().prescriptions;
  const [handledFocus, setHandledFocus] = useState<string | undefined>(
    undefined,
  );
  const [excluded, setExcluded] = useState<readonly string[]>(
    defaultExcludedDrugLabels,
  );
  const [search, setSearch] = useState("");
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
      render: (row) => <CopyButton value={row.nre} label="Copia NRE" />,
    },
    {
      accessor: "azioni",
      title: "Azioni",
      render: (row) => (
        <Group gap={2} wrap="nowrap">
          <PdfLink documentRef={row.fse.documentRef} />
        </Group>
      ),
    },
  ];

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
        <Button
          variant="light"
          leftSection={<RefreshCw size={16} />}
          onClick={reload}
        >
          Aggiorna
        </Button>
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
          maxHeight="75vh"
          idAccessor="nre"
          records={visible}
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
          customRowAttributes={(row) => ({
            id: focusId("farmaci", row.nre),
            tabIndex: -1,
            "data-focused":
              row.nre.toUpperCase() === focus ? "true" : undefined,
          })}
          rowStyle={(row) =>
            row.nre.toUpperCase() === focus
              ? { backgroundColor: focusedBg }
              : undefined
          }
        />
      ) : null}
    </section>
  );
}
