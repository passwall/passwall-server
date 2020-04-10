import * as React from "react"
import { Modal, Button } from "antd"
import { Form, FormItem, Input } from "formik-antd"
import { Formik } from "formik"
import { GlobalOutlined, UserOutlined, LockOutlined } from "@ant-design/icons"
import * as Yup from "yup"

const LoginSchema = Yup.object().shape({
  URL: Yup.string().required("Required"),
  Username: Yup.string().required("Required"),
  Password: Yup.string()
  // .min(6, "Too Short!")
  // .max(128, "Too Long!")
  // .required("Required"),
})

function NewForm({ visible, loading, onClose, onSubmit }) {
  const formRef = React.useRef()

  const onTriggerSubmit = () => {
    if (!formRef.current) return
    formRef.current.handleSubmit()
  }

  return (
    <Modal
      title="New Pass"
      visible={visible}
      closable={false}
      maskClosable={false}
      destroyOnClose={true}
      footer={[
        <Button key="close" shape="round" onClick={onClose}>
          Cancel
        </Button>,
        <Button
          key="save"
          shape="round"
          type="primary"
          loading={loading}
          onClick={onTriggerSubmit}
        >
          Save
        </Button>
      ]}
    >
      <Formik
        innerRef={formRef}
        initialValues={{ URL: "", Username: "", Password: "" }}
        validationSchema={LoginSchema}
        onSubmit={onSubmit}
      >
        {() => (
          <Form layout="vertical">
            <FormItem label="URL" name="URL" required={true}>
              <Input
                name="URL"
                prefix={<GlobalOutlined />}
                placeholder="https://example.com"
              />
            </FormItem>

            <FormItem label="Username" name="Username" required={true}>
              <Input
                name="Username"
                prefix={<UserOutlined />}
                placeholder="Username or email"
              />
            </FormItem>

            {/* <FormItem label="Password" name="Password" required={true}>
              <Input.Password
                name="Password"
                prefix={<LockOutlined />}
                placeholder="input placeholder"
              />
            </FormItem> */}
          </Form>
        )}
      </Formik>
    </Modal>
  )
}

export default NewForm
