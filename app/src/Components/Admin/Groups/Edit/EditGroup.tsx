import React, { useEffect, useState } from 'react';
import Title from '../../../../Shared/Components/Font/Title';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useParams } from 'react-router-dom';
import { AdminGroupResource, type Group, GroupSchema } from '../model/group';
import { mapObjects } from '../../../../config/data/mapper/mapper';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { Accordions, type AccordionData } from '../../../../Shared/Components/Accordion';
import { Switch } from '../../../../Shared/Components/Switch';
import Scopes from '../../../Auth/Scopes/Partials/Scopes.tsx';
import { Users } from './Partials/Users';
import { Submit } from '../../../../Shared/Components/Button';
import { FormInput, FormTextarea } from '../../../../Shared/Components/Form';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

export const EditGroup: React.FC = () => {
  useBreadcrumbItems([{ label: 'Admin' }, { label: 'Groups', href: '/admin/groups' }, { label: 'Edit' }]);
  const { id } = useParams<{ id: string }>();
  const [fetching, setFetching] = useState(true);
  const [loading, setLoading] = useState(false);

  const [title, setTitle] = useState('');
  const [groupDescription, setGroupDescription] = useState('');
  const [isActive, setIsActive] = useState(true);

  const { get, patch } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();

  const fetchGroup = React.useCallback(async () => {
    if (!id) return;
    setFetching(true);
    try {
      const response = await get<{ message: unknown }>(`admin/{${AdminGroupResource}}/{id:${id}}`);
      const rawGroup = response?.message || response;

      if (rawGroup) {
        const mappedGroups = mapObjects(GroupSchema, [rawGroup]) as unknown as Group[];
        const group = mappedGroups[0];
        setTitle(group.title || '');
        setGroupDescription(group.groupDescription || '');
        setIsActive(group.isActive ?? true);
      } else {
        errorMessage('Group not found');
      }

    } catch (err: unknown) {
      let errorMsg = 'Failed to fetch group data';
      if (err instanceof Error) {
        errorMsg = err.message || errorMsg;
      }
      errorMessage(errorMsg);
    } finally {
      setFetching(false);
    }
  }, [id, get, errorMessage]);

  useEffect(() => {
    void fetchGroup();
  }, [fetchGroup]);

  const handlePatch = async (payload: Record<string, unknown>, sectionName: string) => {
    setLoading(true);
    try {
      await patch(`admin/{${AdminGroupResource}}/{id:${id}}`, payload);
      successMessage(`${sectionName} updated successfully!`);
      await fetchGroup();
    } catch (err: unknown) {
      let errorMsg = `Failed to update ${sectionName}`;
      if (err instanceof Error) {
        errorMsg = err.message || errorMsg;
      }
      errorMessage(errorMsg);
    } finally {
      setLoading(false);
    }
  };

  const handleDetailsSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    void handlePatch({ title, group_description: groupDescription }, 'Details');
  };

  const accordionItems: AccordionData[] = [
    {
      id: 'details',
      title: 'Details',
      content: (
        <form onSubmit={handleDetailsSubmit} className="space-y-4">
          <FormInput
            id="title"
            label="Title"
            value={title}
            onChange={setTitle}
            required
          />
          <FormTextarea
            id="groupDescription"
            label="Description"
            value={groupDescription}
            onChange={setGroupDescription}
          />
          <div className="flex justify-end">
            <Submit
              loading={loading}
              loadingText="Saving..."
              disabled={!title}
              label="Update Details"
            />
          </div>
        </form>
      )
    },
    {
      id: 'active',
      title: 'Active Status',
      content: (
        <div className="space-y-6 py-2">
          <Switch
            id="is_active"
            checked={isActive}
            onChange={checked => {
              setIsActive(checked);
              void handlePatch({ is_active: checked }, 'Active Status');
            }}
            label="Active Group"
            disabled={loading}
          />
        </div>
      )
    },
    {
      id: 'scopes',
      title: 'Group Scopes',
      content: id ? <Scopes resourceKey={'group_id'} resourceName={'admin_group_acl'} entityId={id} /> : null
    }
  ];

  if (fetching) {
    return (
      <div className="max-w-full">
        <Title>Edit Admin Group</Title>
        <div className="mt-6 flex items-center justify-center p-6">
          <svg className="animate-spin h-6 w-6 text-indigo-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z"></path>
          </svg>
          <span className="ml-3 text-gray-500">Loading group data...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-full">
      <Title>Edit Admin Group</Title>

      {title && (
        <div className="mt-6">
          <p className="text-sm font-medium text-gray-500 mb-1">Group Title</p>
          <p className="text-xl font-bold text-gray-700 dark:text-gray-300 tracking-tight">{title}</p>
        </div>
      )}

      <div className="mt-6 space-y-4">
        <Accordions items={accordionItems} initialExpandedId="details" />
      </div>

      {id && (
        <div className="mt-8 pt-8">
          <Users entityId={id} />
        </div>
      )}
    </div>
  );
};

export default EditGroup;
