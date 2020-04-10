import * as React from "react"
import { Table, Typography, Input } from "antd"
import Highlighter from "react-highlight-words"

const { Paragraph } = Typography

function PassTable({ loading, data }) {
  const [searchText, setSearchText] = React.useState("")
  const [dataTable, setDataTable] = React.useState([])

  React.useEffect(() => {
    setDataTable(
      searchText.length
        ? data.filter((pass) =>
            pass.URL.toString()
              .toLocaleLowerCase()
              .includes(searchText.toLocaleLowerCase())
          )
        : data
    )
  }, [searchText])

  React.useEffect(() => {
    setDataTable(data)
  }, [data])

  const columns = [
    {
      title: "Url",
      dataIndex: "URL",
      ellipsis: true,
      sorter: (a, b) => a.URL.localeCompare(b.URL),
      sortDirections: ["descend", "ascend"],
      render: (text) => {
        console.log("render", text)
        return text
      },
      render: (text) => (
        <Highlighter
          highlightStyle={{ backgroundColor: "#ffc069", padding: 0 }}
          searchWords={[searchText]}
          autoEscape
          textToHighlight={text.toString()}
        />
      )
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

  return (
    <div>
      <Input
        style={{ marginBottom: 20 }}
        placeholder="Search"
        value={searchText}
        onChange={(e) => setSearchText(e.target.value)}
      />

      <Table
        size="small"
        loading={loading}
        columns={columns}
        rowKey="ID"
        dataSource={dataTable}
      />
    </div>
  )
}

export default PassTable
