"use client"

import { useGetCodeContextQuery } from "@/store/services"
import { GitBranch, Code, ExternalLink, Clock, Settings } from "lucide-react"
import Link from "next/link"

interface CodeContextPanelProps {
  projectId: string
  serviceId: string
  serviceName?: string
}

export function CodeContextPanel({ projectId, serviceId, serviceName }: CodeContextPanelProps) {
  const { data: ctx, isLoading } = useGetCodeContextQuery(
    { projectId, serviceId },
    { skip: !projectId || !serviceId }
  )

  if (isLoading) {
    return (
      <div className="animate-pulse space-y-2">
        <div className="h-4 bg-slate-200 dark:bg-slate-700 rounded w-32" />
        <div className="h-3 bg-slate-100 dark:bg-slate-800 rounded w-full" />
      </div>
    )
  }

  // No repo configured
  if (!ctx?.repoUrl) {
    return (
      <div className="p-4 rounded-xl border border-dashed border-slate-300 dark:border-slate-600 bg-slate-50/50 dark:bg-slate-800/30">
        <div className="flex items-start gap-3">
          <Code className="w-4 h-4 text-slate-400 flex-shrink-0 mt-0.5" />
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium text-slate-600 dark:text-slate-400">
              Code context not configured
            </p>
            <p className="text-xs text-slate-400 dark:text-slate-500 mt-0.5">
              Link a Git repository to <strong>{serviceName ?? serviceId}</strong> to see which
              recent commit likely caused this fault.
            </p>
            <Link
              href={`/dashboard/projects/${projectId}?tab=code`}
              className="inline-flex items-center gap-1 text-xs text-blue-500 hover:underline mt-2"
            >
              <Settings className="w-3 h-3" />
              Configure repo in Project Settings
            </Link>
          </div>
        </div>
      </div>
    )
  }

  const commits = ctx.recentCommits ?? []

  const FAULT_KEYWORDS = ["error", "fix", "bug", "crash", "fail", "latency", "timeout", "memory", "leak", "exception", "panic", "revert"]

  const hasKeywordMatch = (message: string): boolean =>
    FAULT_KEYWORDS.some((kw) => message.toLowerCase().includes(kw))

  return (
    <div className="space-y-3">
      {/* Repo header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
          <GitBranch className="w-3.5 h-3.5" />
          <a
            href={ctx.repoUrl}
            target="_blank"
            rel="noopener"
            className="hover:text-blue-500 hover:underline truncate max-w-[280px]"
          >
            {ctx.repoUrl.replace("https://github.com/", "")}
          </a>
        </div>
        <a
          href={ctx.repoUrl}
          target="_blank"
          rel="noopener"
          className="text-xs text-slate-400 hover:text-slate-600 dark:hover:text-slate-300"
        >
          <ExternalLink className="w-3 h-3" />
        </a>
      </div>

      {/* Summary */}
      {ctx.summary && (
        <div className="text-xs text-slate-500 dark:text-slate-400 px-1">
          {ctx.summary}
        </div>
      )}

      {/* Recent commits */}
      {commits.length > 0 ? (
        <div className="space-y-2">
          {commits.map((commit, i) => (
            <div key={commit.hash}>
              {/* "Less likely" separator before subsequent commits */}
              {i === 1 && commits.length > 1 && (
                <div className="flex items-center gap-2 mb-2 mt-1">
                  <div className="flex-1 h-px bg-slate-200 dark:bg-slate-700" />
                  <span className="text-[10px] font-medium text-slate-400 dark:text-slate-500 uppercase tracking-wide">
                    Less likely
                  </span>
                  <div className="flex-1 h-px bg-slate-200 dark:bg-slate-700" />
                </div>
              )}
              <div
                className={`p-3 rounded-lg border ${
                  i === 0
                    ? "border-orange-200 dark:border-orange-800 bg-orange-50/50 dark:bg-orange-900/10"
                    : "border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900"
                }`}
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="flex items-start gap-2 min-w-0">
                    <div className="flex flex-col gap-1 flex-shrink-0 mt-0.5">
                      {i === 0 && (
                        <span className="text-[9px] font-bold bg-orange-200 dark:bg-orange-800 text-orange-800 dark:text-orange-200 px-1.5 py-0.5 rounded uppercase tracking-wide">
                          ⚠ Likely cause
                        </span>
                      )}
                      {i > 0 && hasKeywordMatch(commit.message) && (
                        <span className="text-[9px] font-bold bg-yellow-100 dark:bg-yellow-900/40 text-yellow-800 dark:text-yellow-300 px-1.5 py-0.5 rounded uppercase tracking-wide">
                          🔍 Keyword match
                        </span>
                      )}
                    </div>
                    <div className="min-w-0">
                      <p className="text-xs font-medium text-slate-800 dark:text-slate-100 line-clamp-2">
                        {commit.message}
                      </p>
                      <div className="flex items-center gap-2 mt-1 text-[10px] text-slate-400">
                        <span className="font-mono">{commit.hash.slice(0, 7)}</span>
                        <span>·</span>
                        <span>{commit.author}</span>
                        <span>·</span>
                        <span className="flex items-center gap-0.5">
                          <Clock className="w-2.5 h-2.5" />
                          {new Date(commit.timestamp).toLocaleString()}
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
                {commit.changedFiles?.length > 0 && (
                  <div className="mt-2 flex flex-wrap gap-1">
                    {commit.changedFiles.slice(0, 4).map((f) => (
                      <span
                        key={f}
                        className="text-[10px] font-mono px-1.5 py-0.5 bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 rounded"
                      >
                        {f.split("/").pop()}
                      </span>
                    ))}
                    {commit.changedFiles.length > 4 && (
                      <span className="text-[10px] text-slate-400">+{commit.changedFiles.length - 4} more</span>
                    )}
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="text-xs text-slate-400 dark:text-slate-500 text-center py-3">
          No recent commits indexed yet — code-indexer runs on schedule
        </div>
      )}
    </div>
  )
}
