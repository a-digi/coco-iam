import React from 'react';
import { useEffect, useState } from 'react';
import Masonry from '../../../Shared/Components/Masonry/Masonry';
import Title from '../../../Shared/Components/Font/Title';

import {type Standard, StandardSchema} from '../model/standard.ts';
import {mapObjects} from '../../../config/data/mapper/mapper.ts';
import type {ApiCollectionResponse} from '../../../config/data/response/response.ts';
import {get} from '../../../api/get.ts';

const StandardDashboard: React.FC = () => {
  const [data, setData] = useState<Standard[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    get('standards')
      .then((res) => {
        const response = res as ApiCollectionResponse<Standard>;
        if (!response.message || response.message.length === 0) {
          setData([]);
          return;
        }
        const items = mapObjects(StandardSchema, response.message);
        setData(items as unknown as Standard[]);
        setError(null);
      })
      .catch(() => {
        setError('Failed to load dashboard data.');
        setData([]);
      })
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="p-8 text-gray-500">Loading...</div>;
  if (error) return <div className="p-8 text-red-500">{error}</div>;

  return (
    <div>
      <Title>Standard Dashboard</Title>
      <Masonry columns={3} gap={16}>
        {data.map((item) => {
          return (
            <div
              key={item.id}
            >
              <h3 className="text-lg font-semibold mb-2 text-gray-800">{item.title}</h3>
              <p className="text-gray-600">{item.version}</p>
            </div>
          );
        })}
      </Masonry>
    </div>
  );
};

export default StandardDashboard;
