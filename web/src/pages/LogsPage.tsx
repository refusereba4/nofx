import useSWR from 'swr'
import { api } from '../lib/api'
import type { RiskAPICallLogsResponse } from '../types'

const CATEGORY_LABELS: Record<string, string> = {
  positionRisk: 'positionRisk API (/fapi/v2/positionRisk)',
  'openOrders.standard': 'openOrders.standard API (/fapi/v1/openOrders)',
  'openOrders.algo': 'openOrders.algo API (/fapi/v1/algo/openOrders)',
}

export function LogsPage() {
  const { data, error, isLoading } = useSWR<RiskAPICallLogsResponse>(
    'risk-api-call-logs',
    () => api.getRiskAPICallLogs(30),
    {
      refreshInterval: 10000,
      revalidateOnFocus: false,
      dedupingInterval: 8000,
    }
  )

  const items = data?.items ?? []

  return (
    <div className="max-w-[1920px] mx-auto px-6 py-8">
      <div
        className="rounded-xl p-5 mb-6"
        style={{ background: '#11151C', border: '1px solid #2B3139' }}
      >
        <div className="flex items-center justify-between gap-3 flex-wrap">
          <div>
            <h1 className="text-2xl font-bold" style={{ color: '#EAECEF' }}>
              API Call Logs
            </h1>
            <p className="text-sm mt-1" style={{ color: '#848E9C' }}>
              每分钟 API 调用计数（仅新风控路径，最近 30 分钟）
            </p>
          </div>
          <div className="text-sm" style={{ color: '#848E9C' }}>
            {data?.generated_at
              ? `Updated: ${new Date(data.generated_at).toLocaleString()}`
              : '--'}
          </div>
        </div>
      </div>

      <div
        className="rounded-xl overflow-hidden"
        style={{ background: '#0F131A', border: '1px solid #2B3139' }}
      >
        <div className="overflow-auto">
          <table className="w-full min-w-[780px]">
            <thead style={{ background: '#141922' }}>
              <tr>
                <th className="text-left text-xs uppercase tracking-wider px-4 py-3" style={{ color: '#848E9C' }}>
                  Minute
                </th>
                <th className="text-right text-xs uppercase tracking-wider px-4 py-3" style={{ color: '#848E9C' }}>
                  positionRisk.apiCalls
                </th>
                <th className="text-right text-xs uppercase tracking-wider px-4 py-3" style={{ color: '#848E9C' }}>
                  openOrders.standard.apiCalls
                </th>
                <th className="text-right text-xs uppercase tracking-wider px-4 py-3" style={{ color: '#848E9C' }}>
                  openOrders.algo.apiCalls
                </th>
                <th className="text-right text-xs uppercase tracking-wider px-4 py-3" style={{ color: '#848E9C' }}>
                  Total.apiCalls
                </th>
              </tr>
            </thead>
            <tbody>
              {isLoading && (
                <tr>
                  <td colSpan={5} className="px-4 py-8 text-center" style={{ color: '#848E9C' }}>
                    Loading...
                  </td>
                </tr>
              )}
              {error && (
                <tr>
                  <td colSpan={5} className="px-4 py-8 text-center" style={{ color: '#F6465D' }}>
                    Failed to load API logs: {error.message}
                  </td>
                </tr>
              )}
              {!isLoading && !error && items.length === 0 && (
                <tr>
                  <td colSpan={5} className="px-4 py-8 text-center" style={{ color: '#848E9C' }}>
                    No API call logs yet.
                  </td>
                </tr>
              )}
              {!isLoading &&
                !error &&
                [...items].reverse().map((row) => (
                  <tr key={row.minute} style={{ borderTop: '1px solid #1E2329' }}>
                    <td className="px-4 py-3 text-sm font-medium" style={{ color: '#EAECEF' }}>
                      {row.minute}
                    </td>
                    <td className="px-4 py-3 text-sm text-right" style={{ color: '#F0B90B' }}>
                      {row.counts.positionRisk ?? 0}
                    </td>
                    <td className="px-4 py-3 text-sm text-right" style={{ color: '#0ECB81' }}>
                      {row.counts['openOrders.standard'] ?? 0}
                    </td>
                    <td className="px-4 py-3 text-sm text-right" style={{ color: '#2EBDFF' }}>
                      {row.counts['openOrders.algo'] ?? 0}
                    </td>
                    <td className="px-4 py-3 text-sm text-right font-semibold" style={{ color: '#EAECEF' }}>
                      {row.total}
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
      </div>

      <div
        className="rounded-xl p-4 mt-6"
        style={{ background: '#11151C', border: '1px solid #2B3139' }}
      >
        <div className="text-sm" style={{ color: '#848E9C' }}>
          <span style={{ color: '#EAECEF' }}>说明</span>：
          `positionRisk` 列统计的是该分钟调用 `positionRisk` 接口的次数，不是当前持仓数量；
          `total` 是该分钟三类接口调用次数之和。
        </div>
        <div className="text-sm mt-2" style={{ color: '#848E9C' }}>
          Categories:
          {Object.entries(CATEGORY_LABELS).map(([key, label]) => (
            <span key={key} className="ml-3">
              <span style={{ color: '#EAECEF' }}>{key}</span>: {label}
            </span>
          ))}
        </div>
      </div>
    </div>
  )
}
