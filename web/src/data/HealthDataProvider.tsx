import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type JSX,
  type ReactNode,
} from "react";

import { notifications } from "@mantine/notifications";

import {
  api,
  type AppointmentsBody,
  type PrescriptionsBody,
} from "../api/client";
import { useSession } from "../session/useSession";
import {
  HealthDataContext,
  type HealthData,
  type HealthError,
} from "./healthContext";
import { buildIndexes } from "./indexes";
import { refreshPortals, type RefreshScope } from "./refresh";

interface Props {
  children: ReactNode;
}

interface Result<T> {
  sessionKey: string;
  requestId: string;
  data: T | undefined;
  error: HealthError | undefined;
}

function appointmentsError(status: number): HealthError {
  if (status === 401) {
    return "unauthorized";
  }
  if (status === 409) {
    return "needs_gestione";
  }
  return "unavailable";
}

export function HealthDataProvider({ children }: Props): JSX.Element {
  const { session } = useSession();
  const [appointmentReloads, setAppointmentReloads] = useState(0);
  const [prescriptionReloads, setPrescriptionReloads] = useState(0);
  const [appointmentResult, setAppointmentResult] = useState<
    Result<AppointmentsBody> | undefined
  >(undefined);
  const [prescriptionResult, setPrescriptionResult] = useState<
    Result<PrescriptionsBody> | undefined
  >(undefined);
  const [refreshCalls, setRefreshCalls] = useState(0);

  const cupState =
    session?.portals?.find((portal) => portal.portal === "cup")?.state ?? "";
  const gestioneReady = session?.gestioneReady ?? false;
  const portalKey =
    session?.portals?.map((p) => `${p.portal}:${p.state}`).join("|") ?? "";
  const appointmentSessionKey = `${cupState}|${String(gestioneReady)}`;
  const appointmentId = `${appointmentSessionKey}|${String(appointmentReloads)}`;
  const prescriptionId = `${portalKey}#${String(prescriptionReloads)}`;

  useEffect(() => {
    const controller = new AbortController();
    const run = async (): Promise<void> => {
      try {
        const { data, response } = await api.GET("/api/appointments", {
          signal: controller.signal,
        });
        if (controller.signal.aborted) {
          return;
        }
        if (data !== undefined) {
          setAppointmentResult({
            sessionKey: appointmentSessionKey,
            requestId: appointmentId,
            data,
            error: undefined,
          });
          return;
        }
        setAppointmentResult({
          sessionKey: appointmentSessionKey,
          requestId: appointmentId,
          data: undefined,
          error: appointmentsError(response.status),
        });
      } catch {
        if (controller.signal.aborted) {
          return;
        }
        setAppointmentResult({
          sessionKey: appointmentSessionKey,
          requestId: appointmentId,
          data: undefined,
          error: "unavailable",
        });
      }
    };
    void run();
    return () => {
      controller.abort();
    };
  }, [appointmentId, appointmentSessionKey]);

  useEffect(() => {
    const controller = new AbortController();
    const run = async (): Promise<void> => {
      try {
        const { data, response } = await api.GET("/api/prescriptions", {
          signal: controller.signal,
        });
        if (controller.signal.aborted) {
          return;
        }
        if (data !== undefined) {
          setPrescriptionResult({
            sessionKey: portalKey,
            requestId: prescriptionId,
            data,
            error: undefined,
          });
          return;
        }
        setPrescriptionResult({
          sessionKey: portalKey,
          requestId: prescriptionId,
          data: undefined,
          error: response.status === 401 ? "unauthorized" : "unavailable",
        });
      } catch {
        if (controller.signal.aborted) {
          return;
        }
        setPrescriptionResult({
          sessionKey: portalKey,
          requestId: prescriptionId,
          data: undefined,
          error: "unavailable",
        });
      }
    };
    void run();
    return () => {
      controller.abort();
    };
  }, [prescriptionId, portalKey]);

  const reloadAppointments = useCallback((): void => {
    setAppointmentReloads((count) => count + 1);
  }, []);

  const reloadPrescriptions = useCallback((): void => {
    setPrescriptionReloads((count) => count + 1);
  }, []);

  const refresh = useCallback(
    async (scope: RefreshScope): Promise<void> => {
      setRefreshCalls((count) => count + 1);
      try {
        const ok = await refreshPortals(scope);
        if (!ok) {
          notifications.show({
            color: "red",
            message: "Aggiornamento non riuscito",
          });
          return;
        }
        reloadAppointments();
        reloadPrescriptions();
      } catch {
        notifications.show({
          color: "red",
          message: "Aggiornamento non riuscito",
        });
      } finally {
        setRefreshCalls((count) => count - 1);
      }
    },
    [reloadAppointments, reloadPrescriptions],
  );

  const currentAppointments =
    appointmentResult?.sessionKey === appointmentSessionKey
      ? appointmentResult
      : undefined;
  const currentPrescriptions =
    prescriptionResult?.sessionKey === portalKey
      ? prescriptionResult
      : undefined;
  const reloading =
    (currentAppointments !== undefined &&
      currentAppointments.requestId !== appointmentId) ||
    (currentPrescriptions !== undefined &&
      currentPrescriptions.requestId !== prescriptionId);
  const refreshing = refreshCalls > 0 || reloading;

  const appointmentData = currentAppointments?.data;
  const prescriptionData = currentPrescriptions?.data;
  const indexes = useMemo(
    () => buildIndexes(appointmentData, prescriptionData),
    [appointmentData, prescriptionData],
  );

  const value = useMemo<HealthData>(
    () => ({
      appointments: {
        data: appointmentData,
        error: currentAppointments?.error,
        loading: currentAppointments === undefined,
        reload: reloadAppointments,
      },
      prescriptions: {
        data: prescriptionData,
        error: currentPrescriptions?.error,
        loading: currentPrescriptions === undefined,
        reload: reloadPrescriptions,
      },
      ...indexes,
      refreshing,
      refresh,
    }),
    [
      appointmentData,
      currentAppointments,
      reloadAppointments,
      prescriptionData,
      currentPrescriptions,
      reloadPrescriptions,
      indexes,
      refreshing,
      refresh,
    ],
  );

  return <HealthDataContext value={value}>{children}</HealthDataContext>;
}
