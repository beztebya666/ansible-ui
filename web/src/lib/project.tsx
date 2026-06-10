// Multi-tenant scope: the "current project". Persisted to localStorage and sent
// as X-Project-Id by api.ts. Changing it refetches every scoped list.
import { createContext, useCallback, useContext, useState, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";

interface ProjectCtx {
  currentId: string;
  setCurrent: (id: string) => void;
}

const Ctx = createContext<ProjectCtx>({ currentId: "", setCurrent: () => {} });
export const useProject = () => useContext(Ctx);

export function ProjectProvider({ children }: { children: ReactNode }) {
  const qc = useQueryClient();
  const [currentId, setId] = useState(() => {
    try {
      return localStorage.getItem("aui.project") || "";
    } catch {
      return "";
    }
  });

  const setCurrent = useCallback(
    (id: string) => {
      setId(id);
      try {
        if (id) localStorage.setItem("aui.project", id);
        else localStorage.removeItem("aui.project");
      } catch {
        /* ignore */
      }
      // Re-fetch everything under the new scope.
      qc.invalidateQueries();
    },
    [qc],
  );

  return <Ctx.Provider value={{ currentId, setCurrent }}>{children}</Ctx.Provider>;
}
