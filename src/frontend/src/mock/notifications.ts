/**
 * 通知的开发期测试数据: mock 以 .ts 作为数据源直接导入, 与 static/*.json (真实应用数据) 职责不同。
 * 接入真实通知源后, 只需把消费方改为从对应数据源取数。
 */

export type Notice = {
  id: string
  label: string
  title: string
  content: string
}

export const notices: Notice[] = [
  {
    id: "notice-1",
    label: "System",
    title: "Bridge session reconnected",
    content:
      "The node child process exited unexpectedly. The listening socket was rebound and the session was re-established automatically, so queued work was resumed without manual action.",
  },
  {
    id: "notice-2",
    label: "Update",
    title: "Version 0.0.2 is available",
    content:
      "A new build is ready to install. It contains the reworked bridge transport layer and the sidebar data provider.",
  },
  {
    id: "notice-3",
    label: "Task",
    title: "Automation run finished",
    content:
      "Run #128 completed with 12 steps: 11 succeeded, 1 skipped. Open the run detail to inspect the recorded traffic.",
  },
  {
    id: "notice-4",
    label: "Alert",
    title: "Capture buffer is almost full",
    content:
      "The capture buffer reached 92% of its configured limit. Older entries are dropped first once the limit is hit; consider exporting or raising it.",
  },
  {
    id: "notice-5",
    label: "System",
    title: "Settings backed up",
    content:
      "Configuration was written to the settings store and a backup copy was created before the write.",
  },
  {
    id: "notice-6",
    label: "Task",
    title: "Proxy profile imported",
    content:
      'Profile "local-dev" was imported with 4 rules. Existing rules with the same name were replaced.',
  },
]
