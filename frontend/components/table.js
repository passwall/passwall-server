import * as React from "react"
import { Table, Typography } from "antd"

const { Paragraph } = Typography

const columns = [
  {
    title: "Url",
    dataIndex: "URL",
    ellipsis: true
  },
  {
    title: "Username",
    dataIndex: "Username"
  },
  {
    title: "Password",
    dataIndex: "Password",
    render: (text) => (
      <Paragraph style={{ marginBottom: 0 }} copyable>
        {text}
      </Paragraph>
    )
  }
]

function PassTable({ loading, data }) {
  return (
    <Table
      size="small"
      loading={loading}
      columns={columns}
      rowKey="ID"
      dataSource={data}
    />
  )
}

export default PassTable
