import * as React from "react"
import { Space, Button, Typography } from "antd"
import { ReloadOutlined, PlusOutlined } from "@ant-design/icons"

const { Title } = Typography

function Header({ loading, onModalOpen = () => {}, onDataRefresh = () => {} }) {
  return (
    <header className="header">
      <Space>
        <Title style={{ marginBottom: 0 }} level={2}>
          GPass
        </Title>

        <Button
          shape="circle"
          loading={loading}
          icon={<ReloadOutlined />}
          onClick={() => onDataRefresh()}
        />
      </Space>

      <Button
        shape="round"
        type="primary"
        icon={<PlusOutlined />}
        onClick={onModalOpen}
      >
        New Pass
      </Button>

      <style jsx>{`
        .header {
          display: grid;
          grid-template-columns: 1fr auto;
          align-items: center;
        }
      `}</style>
    </header>
  )
}

export default Header
