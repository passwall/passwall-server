import { Modal, Form, Input, Button, Radio } from "antd"
import { GlobalOutlined, UserOutlined, LockOutlined } from "@ant-design/icons"

function NewForm({ visible, onNewOk, onNewCancel }) {
  return (
    <Modal
      title="Basic Modal"
      visible={visible}
      onOk={onNewOk}
      onCancel={onNewCancel}
    >
      <Form layout="vertical">
        <Form.Item label="URL">
          <Input
            prefix={<GlobalOutlined />}
            placeholder="https://example.com"
          />
        </Form.Item>

        <Form.Item label="Username">
          <Input prefix={<UserOutlined />} placeholder="Username or email" />
        </Form.Item>

        <Form.Item label="Password">
          <Input prefix={<LockOutlined />} placeholder="input placeholder" />
        </Form.Item>
      </Form>
    </Modal>
  )
}

export default NewForm
